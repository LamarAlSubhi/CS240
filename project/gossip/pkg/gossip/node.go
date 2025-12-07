package gossip

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

// Node represents a gossip participant.
type Node struct {
	cfg   Config
	tx    Transport
	store *Store

	mu    sync.RWMutex
	peers []string

	stop   chan struct{}
	wg     sync.WaitGroup
	aeTick *time.Ticker
}

// NewNode creates a new gossip node with the given configuration and transport.
func NewNode(cfg Config, tx Transport) *Node {
	return &Node{
		cfg:   cfg,
		tx:    tx,
		store: NewStore(cfg.LogPath, cfg.ID), // pass node ID here
		peers: append([]string{}, cfg.Seeds...),
		stop:  make(chan struct{}),
	}
}

// Inject creates a new rumor at this node, logs the injection,
// and inserts it into the local store.
func (n *Node) Inject(id string, body []byte, ttl int) {
	if ttl <= 0 {
		ttl = n.cfg.TTL
	}

	now := time.Now().UnixNano()
	r := Rumor{
		ID:       id,
		TTL:      ttl,
		Body:     body,
		Origin:   n.cfg.ID,
		InjectTS: now,
	}

	// 1) log injection at origin
	n.store.writeLog("inject", r)

	// 2) store it (this will also log "deliver")
	n.store.Add(r)
}

// Start launches the node’s gossip loops.
func (n *Node) Start() {
	n.wg.Add(2)
	go n.gossipLoop()
	go n.antiEntropyLoop()
}

// Close stops the node and cleans up resources.
func (n *Node) Close() {
	close(n.stop)
	n.wg.Wait()
	_ = n.store.Close()
	_ = n.tx.Close()
}

// ------------------------
// Periodic gossip (push)
// ------------------------

func (n *Node) gossipLoop() {
	defer n.wg.Done()
	t := time.NewTicker(n.cfg.Period)
	defer t.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-t.C:
			n.tickOnce()
		}
	}
}

func (n *Node) tickOnce() {
	rs := n.store.ReadyToPush()
	if len(rs) == 0 {
		return
	}

	peers := n.pickPeers(n.cfg.Fanout)
	msg := Msg{Type: MsgPush, From: n.cfg.ID, Rumors: rs}

	log.Printf("[send] to %v type=%s (%d rumors)\n", peers, msg.Type, len(msg.Rumors))
	for _, r := range msg.Rumors {
		log.Printf("    rumor id=%s ttl=%d body=%q\n", r.ID, r.TTL, string(r.Body))
	}

	// one "send" event per outgoing message
	var dummy Rumor
	if len(rs) > 0 {
		dummy = Rumor{ID: rs[0].ID} // just tag the batch with some id
	}

	for _, p := range peers {
		n.store.writeLog("send", dummy)

		if err := n.tx.Send(p, msg); err != nil {
			log.Printf("send to %s: %v", p, err)
		}
	}
}

func (n *Node) pickPeers(k int) []string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if len(n.peers) == 0 {
		return nil
	}
	if k >= len(n.peers) {
		return append([]string{}, n.peers...)
	}
	perm := rand.Perm(len(n.peers))
	out := make([]string, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, n.peers[perm[i]])
	}
	return out
}

// ------------------------
// Message handlers
// ------------------------

func (n *Node) Handle(m Msg, remote string) {
	log.Printf("[recv] from %s type=%s (%d rumors)\n", remote, m.Type, len(m.Rumors))
	for _, r := range m.Rumors {
		log.Printf("    rumor id=%s ttl=%d body=%q\n", r.ID, r.TTL, string(r.Body))
	}
	// log one "recv" event per incoming message
	var dummy Rumor
	if len(m.Rumors) > 0 {
		dummy = m.Rumors[0]
	}
	n.store.writeLog("recv", dummy)


	switch m.Type {
	case MsgPush:
		for _, r := range m.Rumors {
			if r.TTL <= 0 {
				continue
			}
			r.TTL--
			n.store.Add(r)
		}

	case MsgDigest:
		n.handleDigest(m, remote)

	case MsgMissing:
		n.handleMissing(m, remote)

	case MsgFetchRep:
		n.handleFetchRep(m, remote)
	}
}

// ------------------------
// Anti-entropy reconciliation (push–pull)
// ------------------------

func (n *Node) antiEntropyLoop() {
	defer n.wg.Done()
	n.aeTick = time.NewTicker(1 * time.Second) // might need to tweak
	defer n.aeTick.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-n.aeTick.C:
			n.sendDigest()
		}
	}
}

func (n *Node) sendDigest() {
	peers := n.pickPeers(1)
	if len(peers) == 0 {
		return
	}
	d := n.store.Digest()
	if len(d) == 0 {
		return
	}
	msg := Msg{Type: MsgDigest, From: n.cfg.ID, IDs: d}
	log.Printf("[digest] sending to %s (%d ids)\n", peers[0], len(d))
	_ = n.tx.Send(peers[0], msg)
}

func (n *Node) handleDigest(m Msg, remote string) {
	ours := n.store.Digest()
	missing := diff(ours, m.IDs)
	if len(missing) > 0 {
		log.Printf("[digest] we have %d rumors they lack → requesting from %s\n", len(missing), remote)
		_ = n.tx.Send(remote, Msg{Type: MsgMissing, From: n.cfg.ID, IDs: missing})
	}
}

func (n *Node) handleMissing(m Msg, remote string) {
	rs := n.store.LookupMany(m.IDs)
	if len(rs) == 0 {
		return
	}
	log.Printf("[missing] %s requested %d rumors → sending back\n", remote, len(rs))
	_ = n.tx.Send(remote, Msg{Type: MsgFetchRep, From: n.cfg.ID, Rumors: rs})
}

func (n *Node) handleFetchRep(m Msg, remote string) {
	for _, r := range m.Rumors {
		if r.TTL <= 0 {
			continue
		}
		n.store.Add(r)
	}
	log.Printf("[fetchrep] received %d rumors from %s\n", len(m.Rumors), remote)
}

// ------------------------
// Small helpers
// ------------------------

func diff(a, b []string) []string {
	bset := make(map[string]struct{}, len(b))
	for _, x := range b {
		bset[x] = struct{}{}
	}
	var out []string
	for _, x := range a {
		if _, ok := bset[x]; !ok {
			out = append(out, x)
		}
	}
	return out
}

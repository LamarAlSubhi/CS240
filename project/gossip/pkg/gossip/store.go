package gossip

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Store keeps track of all rumors a node has seen
// and logs events to a JSONL file for analysis.
type Store struct {
	mu     sync.Mutex
	seen   map[string]Rumor // rumor ID -> Rumor
	newIDs []string         // rumors to push in next tick
	logf   *os.File

	nodeID string // NEW: which node is this store attached to

}

// NewStore initializes the store and opens the log file.
func NewStore(path string, nodeID string) *Store {
	f, _ := os.Create(path)
	return &Store{
		seen: map[string]Rumor{},
		logf: f,
		nodeID: nodeID,
	}
}

// Close closes the log file (called on shutdown).
func (s *Store) Close() error {
	if s.logf != nil {
		return s.logf.Close()
	}
	return nil
}

// Add inserts a rumor if unseen.
// Returns true if it's new (not duplicate).
func (s *Store) Add(r Rumor) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.seen[r.ID]; ok {
		return false
	}
	s.seen[r.ID] = r
	s.newIDs = append(s.newIDs, r.ID)
	
	// "deliver" = this node has just learned this rumor
	s.writeLog("deliver", r)
	return true
}

// ReadyToPush returns rumors recently added (un-gossiped yet)
// and clears the list. Called by the gossip loop each tick.
func (s *Store) ReadyToPush() []Rumor {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.newIDs) == 0 {
		return nil
	}
	out := make([]Rumor, 0, len(s.newIDs))
	for _, id := range s.newIDs {
		out = append(out, s.seen[id])
	}
	s.newIDs = s.newIDs[:0]
	return out
}

// Digest returns all rumor IDs known by this node.
// Used for anti-entropy and metrics.
func (s *Store) Digest() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.seen))
	for id := range s.seen {
		ids = append(ids, id)
	}
	return ids
}

// DigestCount returns how many rumors this node knows.
func (s *Store) DigestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}

// writeLog appends a JSONL record of an event to the log file.
func (s *Store) writeLog(ev string, r Rumor) {
	if s.logf == nil {
		return
	}

	type LogRec struct {
		TS       int64  `json:"ts"`                 // event time (this node)
		Node     string `json:"node"`               // which node logged this
		Ev       string `json:"ev"`                 // "inject" | "deliver" | ...
		ID       string `json:"id"`                 // rumor id
		Sz       int    `json:"sz"`                 // body size
		Origin   string `json:"origin,omitempty"`   // origin node (if set)
		InjectTS int64  `json:"inject_ts,omitempty"`// injection time at origin (if set)
	}

	rec := LogRec{
		TS:       time.Now().UnixNano(),
		Node:     s.nodeID,
		Ev:       ev,
		ID:       r.ID,
		Sz:       len(r.Body),
		Origin:   r.Origin,
		InjectTS: r.InjectTS,
	}

	b, _ := json.Marshal(rec)
	_, _ = s.logf.Write(append(b, '\n'))
}

// LookupMany returns the Rumor objects for the given IDs (skips unknown IDs).
func (s *Store) LookupMany(ids []string) []Rumor {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Rumor, 0, len(ids))
	for _, id := range ids {
		if r, ok := s.seen[id]; ok {
			out = append(out, r)
		}
	}
	return out
}

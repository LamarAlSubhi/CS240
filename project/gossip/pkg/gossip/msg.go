package gossip

type MsgType string

const (
	MsgPush     MsgType = "push"
	MsgDigest   MsgType = "digest"
	MsgMissing  MsgType = "missing"
	MsgFetchRep MsgType = "fetchrep"
)

// Rumor represents one piece of gossip information.
type Rumor struct {
	ID   string `json:"id"`
	TTL  int    `json:"ttl"`
	Body []byte `json:"body,omitempty"`

	// filled only at the origin node
	Origin   string `json:"origin,omitempty"`    // node that first injected it
	InjectTS int64  `json:"inject_ts,omitempty"` // time of injection (Unix nanos)
}
// Msg is the top-level message exchanged between nodes.
type Msg struct {
	Type   MsgType  `json:"type"`
	From   string   `json:"from"`
	Rumors []Rumor  `json:"rumors,omitempty"`
	IDs    []string `json:"ids,omitempty"`
}

package gossip

import "time"

type Config struct {
	ID          string
	Bind        string        // e.g., ":9001"
	Seeds       []string      // e.g., ["127.0.0.1:9002"]
	Period      time.Duration // e.g., 200ms
	Fanout      int           // e.g., 3
	TTL         int           // e.g., 8
	MetricsAddr string        // e.g., ":9081"
	LogPath     string        // e.g., "/tmp/gossip-1.jsonl"
	AEInterval  time.Duration // for later; unused in MVP
}

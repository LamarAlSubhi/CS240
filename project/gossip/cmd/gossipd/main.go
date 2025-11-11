package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"gossip/pkg/gossip"
)

func main() {
	id := flag.String("id", "", "node id")
	bind := flag.String("bind", "0.0.0.0:9000", "udp bind")
	seeds := flag.String("seeds", "", "comma-separated seed addrs")
	period := flag.Duration("period", 200*time.Millisecond, "gossip period")
	fanout := flag.Int("fanout", 3, "fanout")
	ttl := flag.Int("ttl", 8, "ttl")
	metrics := flag.String("metrics-addr", "0.0.0.0:9090", "http addr")
	logPath := flag.String("log", "/tmp/gossip.jsonl", "log path")
	flag.Parse()

	cfg := gossip.Config{
		ID: *id, Bind: *bind,
		Seeds:       splitCSV(*seeds),
		Period:      *period,
		Fanout:      *fanout,
		TTL:         *ttl,
		MetricsAddr: *metrics,
		LogPath:     *logPath,
	}

	tx := gossip.NewUDPTransport()

	// Create node first; then start listening with a closure that calls n.Handle
	n := gossip.NewNode(cfg, tx)
	if err := tx.Listen(cfg.Bind, func(m gossip.Msg, remote string) { n.Handle(m, remote) }); err != nil {
		log.Fatal(err)
	}

	n.Start()
	log.Printf("[gossipd %s] udp=%s http=%s seeds=%v", cfg.ID, cfg.Bind, cfg.MetricsAddr, cfg.Seeds)
	if err := n.ServeHTTP(); err != nil {
		log.Fatal(err)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

package gossip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
)

var injected uint64

func (n *Node) ServeHTTP() error {
	if n.cfg.MetricsAddr == "" {
		return nil
	}

	http.HandleFunc("/inject", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			id = fmt.Sprintf("%s-%d", n.cfg.ID, atomic.AddUint64(&injected, 1))
		}
		body := []byte(r.URL.Query().Get("body"))
		ttl, _ := strconv.Atoi(r.URL.Query().Get("ttl"))
		if ttl <= 0 {
			ttl = n.cfg.TTL
		}
		// using the helper, we log "inject" + origin + inject_ts
		n.Inject(id, body, ttl)
		_, _ = w.Write([]byte("ok\n"))
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": n.cfg.ID, "known": n.store.DigestCount(),
		})
	})

	return http.ListenAndServe(n.cfg.MetricsAddr, nil)
}

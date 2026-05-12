// inbox-echo is a tiny HTTP server for debugging WABOT_INBOUND_URL.
//
// Run:
//
//	go run ./cmd/inbox-echo
//
// Then in wabot.env:
//
//	WABOT_INBOUND_URL=http://127.0.0.1:9000/whatsapp/inbound
//	WABOT_INBOUND_TOKEN=same-as-INBOX_ECHO_TOKEN   # optional
//
// Env:
//
//	LISTEN_ADDR        default 127.0.0.1:9000
//	INBOX_PATH         default /whatsapp/inbound
//	INBOX_ECHO_TOKEN   if set, require Authorization: Bearer <token>
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9000"
	}
	path := os.Getenv("INBOX_PATH")
	if path == "" {
		path = "/whatsapp/inbound"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	wantTok := os.Getenv("INBOX_ECHO_TOKEN")

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		if wantTok != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			got = strings.TrimSpace(got)
			if got != wantTok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println("--- inbound", time.Now().UTC().Format(time.RFC3339Nano), "---")
		fmt.Println(string(body))
		fmt.Println("--- end ---")
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	})

	log.Printf("inbox-echo listening on http://%s%s (health: /health)\n", addr, path)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

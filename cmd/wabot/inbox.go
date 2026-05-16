package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

const inboxMaxEntries = 200

var (
	inboxMu      sync.Mutex
	inboxEntries []inboundPayload
)

func recordInboundMessage(info types.MessageInfo, text string, msg *waE2E.Message) {
	payload := inboundPayloadFromMessage(info, text, msg)
	inboxMu.Lock()
	defer inboxMu.Unlock()
	inboxEntries = append(inboxEntries, payload)
	if len(inboxEntries) > inboxMaxEntries {
		inboxEntries = inboxEntries[len(inboxEntries)-inboxMaxEntries:]
	}
}

func inboxSnapshot(limit int) []inboundPayload {
	if limit <= 0 {
		limit = 20
	}
	inboxMu.Lock()
	defer inboxMu.Unlock()
	if len(inboxEntries) == 0 {
		return []inboundPayload{}
	}
	start := len(inboxEntries) - limit
	if start < 0 {
		start = 0
	}
	out := make([]inboundPayload, len(inboxEntries[start:]))
	copy(out, inboxEntries[start:])
	return out
}

func handleInboxRecent(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	messages := inboxSnapshot(limit)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"messages": messages,
		"count":    len(messages),
		"note":     "Recent inbound messages observed by wabot. Does not include WhatsApp app unread badge counts.",
	})
}

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mau.fi/whatsmeow/types"
)

const apiTimeout = 30 * time.Second

func requireReadyClient(w http.ResponseWriter) bool {
	if client == nil || !client.IsLoggedIn() || !client.IsConnected() {
		http.Error(w, "whatsapp client not ready", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func handleContactsLookup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phones []string `json:"phones"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Phones) == 0 {
		http.Error(w, "missing 'phones'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	results, err := client.IsOnWhatsApp(ctx, req.Phones)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(results))
	for _, item := range results {
		out = append(out, map[string]any{
			"jid":    item.JID.String(),
			"query":  item.Query,
			"is_on":  item.IsIn,
			"verified_name": func() string {
				if item.VerifiedName != nil {
					return item.VerifiedName.Details.GetVerifiedName()
				}
				return ""
			}(),
		})
	}
	writeJSON(w, map[string]any{"results": out})
}

func handleGroupsList(w http.ResponseWriter, r *http.Request) {
	if !requireReadyClient(w) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		if g == nil {
			continue
		}
		out = append(out, map[string]any{
			"jid":                 g.JID.String(),
			"name":                g.GroupName.Name,
			"participant_count":   g.ParticipantCount,
			"announce":            g.GroupAnnounce.IsAnnounce,
			"locked":              g.GroupLocked.IsLocked,
			"created_at":          g.GroupCreated.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, map[string]any{"groups": out, "count": len(out)})
}

func handleChatsRead(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chat       string   `json:"chat"`
		Sender     string   `json:"sender"`
		MessageIDs []string `json:"message_ids"`
		Timestamp  string   `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Chat == "" || len(req.MessageIDs) == 0 {
		http.Error(w, "missing 'chat' or 'message_ids'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}

	chatJID, err := resolveJID(req.Chat)
	if err != nil {
		http.Error(w, "bad chat: "+err.Error(), http.StatusBadRequest)
		return
	}
	senderJID := chatJID
	if req.Sender != "" {
		senderJID, err = resolveJID(req.Sender)
		if err != nil {
			http.Error(w, "bad sender: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	ts := time.Now().UTC()
	if req.Timestamp != "" {
		parsed, parseErr := time.Parse(time.RFC3339, req.Timestamp)
		if parseErr != nil {
			http.Error(w, "bad timestamp: "+parseErr.Error(), http.StatusBadRequest)
			return
		}
		ts = parsed
	}
	ids := make([]types.MessageID, len(req.MessageIDs))
	copy(ids, req.MessageIDs)

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	if err := client.MarkRead(ctx, ids, ts, chatJID, senderJID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "chat": chatJID.String(), "marked": len(ids)})
}

func handlePresenceTyping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To    string `json:"to"`
		State string `json:"state"`
		Media string `json:"media"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.To == "" {
		http.Error(w, "missing 'to'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	jid, err := resolveJID(req.To)
	if err != nil {
		http.Error(w, "bad recipient: "+err.Error(), http.StatusBadRequest)
		return
	}
	state := types.ChatPresenceComposing
	switch req.State {
	case "", "composing", "typing":
		state = types.ChatPresenceComposing
	case "paused":
		state = types.ChatPresencePaused
	default:
		http.Error(w, "unsupported state", http.StatusBadRequest)
		return
	}
	media := types.ChatPresenceMedia("")
	if req.Media == "audio" {
		media = types.ChatPresenceMediaAudio
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	if err := client.SendChatPresence(ctx, jid, state, media); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "to": jid.String(), "state": string(state)})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

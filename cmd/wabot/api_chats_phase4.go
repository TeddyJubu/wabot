package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
)

func chatJIDFromPath(w http.ResponseWriter, r *http.Request) (types.JID, bool) {
	jidRaw := strings.TrimSpace(r.PathValue("jid"))
	if jidRaw == "" {
		http.Error(w, "missing chat jid in path", http.StatusBadRequest)
		return types.EmptyJID, false
	}
	jid, err := resolveJID(jidRaw)
	if err != nil {
		http.Error(w, "bad jid: "+err.Error(), http.StatusBadRequest)
		return types.EmptyJID, false
	}
	return jid, true
}

func sendAppStatePatch(w http.ResponseWriter, patch appstate.PatchInfo) bool {
	if !requireReadyClient(w) {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	if err := client.SendAppState(ctx, patch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	return true
}

func handleChatMute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mute          *bool `json:"mute"`
		DurationHours int   `json:"duration_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Mute == nil {
		http.Error(w, "missing 'mute'", http.StatusBadRequest)
		return
	}
	jid, ok := chatJIDFromPath(w, r)
	if !ok {
		return
	}
	var patch appstate.PatchInfo
	if *req.Mute {
		dur := time.Duration(req.DurationHours) * time.Hour
		patch = appstate.BuildMute(jid, true, dur)
	} else {
		patch = appstate.BuildMute(jid, false, 0)
	}
	if !sendAppStatePatch(w, patch) {
		return
	}
	writeJSON(w, map[string]any{"ok": true, "chat": jid.String(), "muted": *req.Mute})
}

func handleChatArchive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Archive *bool `json:"archive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Archive == nil {
		http.Error(w, "missing 'archive'", http.StatusBadRequest)
		return
	}
	jid, ok := chatJIDFromPath(w, r)
	if !ok {
		return
	}
	patch := appstate.BuildArchive(jid, *req.Archive, time.Time{}, nil)
	if !sendAppStatePatch(w, patch) {
		return
	}
	writeJSON(w, map[string]any{"ok": true, "chat": jid.String(), "archived": *req.Archive})
}

func handleChatPin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pin *bool `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Pin == nil {
		http.Error(w, "missing 'pin'", http.StatusBadRequest)
		return
	}
	jid, ok := chatJIDFromPath(w, r)
	if !ok {
		return
	}
	patch := appstate.BuildPin(jid, *req.Pin)
	if !sendAppStatePatch(w, patch) {
		return
	}
	writeJSON(w, map[string]any{"ok": true, "chat": jid.String(), "pinned": *req.Pin})
}

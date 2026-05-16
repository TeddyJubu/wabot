package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		Participants []string `json:"participants"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "missing 'name'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}

	participants := make([]types.JID, 0, len(req.Participants))
	for _, phone := range req.Participants {
		jid, err := resolveJID(phone)
		if err != nil {
			http.Error(w, "bad participant: "+err.Error(), http.StatusBadRequest)
			return
		}
		participants = append(participants, jid)
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	info, err := client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         req.Name,
		Participants: participants,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "group": groupInfoToMap(info)})
}

func handleGroupInfo(w http.ResponseWriter, r *http.Request) {
	jidRaw := strings.TrimSpace(r.PathValue("jid"))
	if jidRaw == "" {
		http.Error(w, "missing group jid in path", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	groupJID, err := resolveJID(jidRaw)
	if err != nil {
		http.Error(w, "bad jid: "+err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	info, err := client.GetGroupInfo(ctx, groupJID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "group": groupInfoToMap(info)})
}

func handleGroupInvite(w http.ResponseWriter, r *http.Request) {
	jidRaw := strings.TrimSpace(r.PathValue("jid"))
	if jidRaw == "" {
		http.Error(w, "missing group jid in path", http.StatusBadRequest)
		return
	}
	var req struct {
		Reset bool `json:"reset"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if !requireReadyClient(w) {
		return
	}
	groupJID, err := resolveJID(jidRaw)
	if err != nil {
		http.Error(w, "bad jid: "+err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	link, err := client.GetGroupInviteLink(ctx, groupJID, req.Reset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "jid": groupJID.String(), "invite_link": link})
}

func handleGroupJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteLink string `json:"invite_link"`
		Code       string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(req.InviteLink)
	if code == "" {
		code = strings.TrimSpace(req.Code)
	}
	if code == "" {
		http.Error(w, "missing 'invite_link' or 'code'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	jid, err := client.JoinGroupWithLink(ctx, code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "jid": jid.String()})
}

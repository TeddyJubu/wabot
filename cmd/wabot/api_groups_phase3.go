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

func parseGroupJIDFromPath(w http.ResponseWriter, jidRaw string) (types.JID, bool) {
	jidRaw = strings.TrimSpace(jidRaw)
	if jidRaw == "" {
		http.Error(w, "missing group jid in path", http.StatusBadRequest)
		return types.JID{}, false
	}
	groupJID, err := resolveJID(jidRaw)
	if err != nil {
		http.Error(w, "bad jid: "+err.Error(), http.StatusBadRequest)
		return types.JID{}, false
	}
	return groupJID, true
}

func handleGroupUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     *string `json:"name"`
		Topic    *string `json:"topic"`
		Announce *bool   `json:"announce"`
		Locked   *bool   `json:"locked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == nil && req.Topic == nil && req.Announce == nil && req.Locked == nil {
		http.Error(w, "provide at least one of name, topic, announce, locked", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	groupJID, ok := parseGroupJIDFromPath(w, r.PathValue("jid"))
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	if req.Name != nil {
		if err := client.SetGroupName(ctx, groupJID, strings.TrimSpace(*req.Name)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if req.Topic != nil {
		if err := client.SetGroupTopic(ctx, groupJID, "", "", strings.TrimSpace(*req.Topic)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if req.Announce != nil {
		if err := client.SetGroupAnnounce(ctx, groupJID, *req.Announce); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if req.Locked != nil {
		if err := client.SetGroupLocked(ctx, groupJID, *req.Locked); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	info, err := client.GetGroupInfo(ctx, groupJID)
	if err != nil {
		writeJSON(w, map[string]any{"ok": true, "jid": groupJID.String(), "updated": true})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "jid": groupJID.String(), "group": groupInfoToMap(info)})
}

func handleGroupParticipants(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Participants []string `json:"participants"`
		Action       string   `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "add"
	}
	var change whatsmeow.ParticipantChange
	switch action {
	case "add":
		change = whatsmeow.ParticipantChangeAdd
	case "remove":
		change = whatsmeow.ParticipantChangeRemove
	case "promote":
		change = whatsmeow.ParticipantChangePromote
	case "demote":
		change = whatsmeow.ParticipantChangeDemote
	default:
		http.Error(w, "action must be add, remove, promote, or demote", http.StatusBadRequest)
		return
	}
	if len(req.Participants) == 0 {
		http.Error(w, "missing 'participants'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	groupJID, ok := parseGroupJIDFromPath(w, r.PathValue("jid"))
	if !ok {
		return
	}

	jids := make([]types.JID, 0, len(req.Participants))
	for _, phone := range req.Participants {
		jid, err := resolveJID(phone)
		if err != nil {
			http.Error(w, "bad participant: "+err.Error(), http.StatusBadRequest)
			return
		}
		jids = append(jids, jid)
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	updated, err := client.UpdateGroupParticipants(ctx, groupJID, jids, change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	participants := make([]map[string]any, 0, len(updated))
	for _, p := range updated {
		participants = append(participants, map[string]any{
			"jid":      p.JID.String(),
			"is_admin": p.IsAdmin,
			"is_super": p.IsSuperAdmin,
		})
	}
	writeJSON(w, map[string]any{
		"ok":           true,
		"jid":          groupJID.String(),
		"action":       action,
		"participants": participants,
	})
}

func handleGroupLeave(w http.ResponseWriter, r *http.Request) {
	if !requireReadyClient(w) {
		return
	}
	groupJID, ok := parseGroupJIDFromPath(w, r.PathValue("jid"))
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	if err := client.LeaveGroup(ctx, groupJID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "jid": groupJID.String(), "left": true})
}

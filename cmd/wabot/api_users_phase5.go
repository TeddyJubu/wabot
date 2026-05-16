package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func jidFromPath(w http.ResponseWriter, r *http.Request) (types.JID, bool) {
	jidRaw := strings.TrimSpace(r.PathValue("jid"))
	if jidRaw == "" {
		http.Error(w, "missing jid in path", http.StatusBadRequest)
		return types.EmptyJID, false
	}
	jid, err := resolveJID(jidRaw)
	if err != nil {
		http.Error(w, "bad jid: "+err.Error(), http.StatusBadRequest)
		return types.EmptyJID, false
	}
	return jid, true
}

func userInfoToMap(jid types.JID, info types.UserInfo) map[string]any {
	devices := make([]string, len(info.Devices))
	for i, d := range info.Devices {
		devices[i] = d.String()
	}
	out := map[string]any{
		"jid":        jid.String(),
		"status":     info.Status,
		"picture_id": info.PictureID,
		"devices":    devices,
	}
	if !info.LID.IsEmpty() {
		out["lid"] = info.LID.String()
	}
	if info.VerifiedName != nil && info.VerifiedName.Details != nil {
		if name := info.VerifiedName.Details.GetVerifiedName(); name != "" {
			out["verified_name"] = name
		}
	}
	return out
}

func handleUserInfo(w http.ResponseWriter, r *http.Request) {
	jid, ok := jidFromPath(w, r)
	if !ok {
		return
	}
	if !requireReadyClient(w) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	infos, err := client.GetUserInfo(ctx, []types.JID{jid})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	info, found := infos[jid]
	if !found {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "user": userInfoToMap(jid, info)})
}

func handleUserPicture(w http.ResponseWriter, r *http.Request) {
	jid, ok := jidFromPath(w, r)
	if !ok {
		return
	}
	if !requireReadyClient(w) {
		return
	}
	preview := r.URL.Query().Get("preview") == "true" || r.URL.Query().Get("preview") == "1"
	existingID := strings.TrimSpace(r.URL.Query().Get("picture_id"))

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	params := &whatsmeow.GetProfilePictureParams{
		Preview:    preview,
		ExistingID: existingID,
	}
	if jid.Server == types.GroupServer {
		params.IsCommunity = false
	}

	pic, err := client.GetProfilePictureInfo(ctx, jid, params)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) {
			http.Error(w, "no profile picture", http.StatusNotFound)
			return
		}
		if errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if pic == nil {
		writeJSON(w, map[string]any{
			"ok":        true,
			"unchanged": true,
			"jid":       jid.String(),
			"picture_id": existingID,
		})
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pic.URL, nil)
	if err != nil {
		http.Error(w, "picture request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "picture fetch: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, "picture fetch: upstream HTTP "+strconv.Itoa(resp.StatusCode), http.StatusBadGateway)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		if preview {
			contentType = "image/jpeg"
		} else {
			contentType = "image/jpeg"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Picture-ID", pic.ID)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = io.Copy(w, resp.Body)
}

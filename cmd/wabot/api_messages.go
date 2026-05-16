package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func parseMessageTarget(chat, sender, messageID string) (types.JID, types.JID, types.MessageID, error) {
	if chat == "" || messageID == "" {
		return types.EmptyJID, types.EmptyJID, "", errBadRequest("missing 'chat' or 'message_id'")
	}
	chatJID, err := resolveJID(chat)
	if err != nil {
		return types.EmptyJID, types.EmptyJID, "", err
	}
	senderJID := types.EmptyJID
	if sender != "" {
		senderJID, err = resolveJID(sender)
		if err != nil {
			return types.EmptyJID, types.EmptyJID, "", err
		}
	}
	return chatJID, senderJID, types.MessageID(messageID), nil
}

type badRequestError struct{ msg string }

func (e badRequestError) Error() string { return e.msg }

func errBadRequest(msg string) error { return badRequestError{msg: msg} }

func handleMessageReact(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chat      string `json:"chat"`
		MessageID string `json:"message_id"`
		Sender    string `json:"sender"`
		Reaction  string `json:"reaction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	chatJID, senderJID, msgID, err := parseMessageTarget(req.Chat, req.Sender, req.MessageID)
	if err != nil {
		if _, ok := err.(badRequestError); ok {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	if !reserveSend(w) {
		return
	}

	reaction := req.Reaction
	if reaction == "" {
		reaction = whatsmeow.RemoveReactionText
	}
	msg := client.BuildReaction(chatJID, senderJID, msgID, reaction)
	sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	resp, sendErr := client.SendMessage(sendCtx, chatJID, msg)
	if sendErr != nil {
		http.Error(w, sendErr.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"ok":        true,
		"chat":      chatJID.String(),
		"message_id": string(msgID),
		"reaction":  reaction,
		"id":        resp.ID,
	})
}

func handleMessageEdit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chat      string `json:"chat"`
		MessageID string `json:"message_id"`
		Text      string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Text == "" {
		http.Error(w, "missing 'text'", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	chatJID, _, msgID, err := parseMessageTarget(req.Chat, "", req.MessageID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !reserveSend(w) {
		return
	}

	editMsg := client.BuildEdit(chatJID, msgID, &waE2E.Message{
		Conversation: proto.String(req.Text),
	})
	sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	resp, sendErr := client.SendMessage(sendCtx, chatJID, editMsg)
	if sendErr != nil {
		http.Error(w, sendErr.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"ok":         true,
		"chat":       chatJID.String(),
		"message_id": string(msgID),
		"id":         resp.ID,
	})
}

func handleMessageRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	messageID := strings.TrimSpace(r.PathValue("id"))
	if messageID == "" {
		http.Error(w, "missing message id in path", http.StatusBadRequest)
		return
	}
	chat := strings.TrimSpace(r.URL.Query().Get("chat"))
	sender := strings.TrimSpace(r.URL.Query().Get("sender"))
	if chat == "" {
		http.Error(w, "missing 'chat' query parameter", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}
	chatJID, senderJID, msgID, err := parseMessageTarget(chat, sender, messageID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !reserveSend(w) {
		return
	}

	revoke := client.BuildRevoke(chatJID, senderJID, msgID)
	sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	resp, sendErr := client.SendMessage(sendCtx, chatJID, revoke)
	if sendErr != nil {
		http.Error(w, sendErr.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"ok":         true,
		"chat":       chatJID.String(),
		"message_id": string(msgID),
		"id":         resp.ID,
	})
}

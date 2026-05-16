package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestHandleReceiptEventWebhook(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("WABOT_RECEIPT_URL", srv.URL)
	t.Setenv("WABOT_WEBHOOK_TIMEOUT_SEC", "3")

	handleReceiptEvent(&events.Receipt{
		MessageSource: types.MessageSource{
			Chat:   types.JID{User: "1", Server: types.DefaultUserServer},
			Sender: types.JID{User: "2", Server: types.DefaultUserServer},
		},
		MessageIDs: []types.MessageID{"abc"},
		Timestamp:  time.Now(),
		Type:       types.ReceiptTypeRead,
	})

	time.Sleep(100 * time.Millisecond)
	if !strings.Contains(body, `"receipt_type":"read"`) {
		t.Fatalf("body %s", body)
	}
}

func TestHandleChatPresenceEventWebhook(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("WABOT_PRESENCE_URL", srv.URL)

	handleChatPresenceEvent(&events.ChatPresence{
		MessageSource: types.MessageSource{
			Chat:   types.JID{User: "1", Server: types.DefaultUserServer},
			Sender: types.JID{User: "2", Server: types.DefaultUserServer},
		},
		State: types.ChatPresenceComposing,
	})

	time.Sleep(100 * time.Millisecond)
	if !strings.Contains(body, `"state":"composing"`) {
		t.Fatalf("body %s", body)
	}
}

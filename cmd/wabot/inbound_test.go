package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestExtractInboundText(t *testing.T) {
	cases := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{"nil", nil, ""},
		{"conversation", &waE2E.Message{Conversation: proto.String(" hi ")}, " hi "},
		{"extended", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("quoted")}}, "quoted"},
		{"image no caption", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}, "[image]"},
		{"image caption", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("pic")}}, "pic"},
		{"doc filename", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("x.pdf")}}, "[document: x.pdf]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractInboundText(tc.msg)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestPostInboundWebhook(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"text":"hello"`) {
			t.Fatalf("body %s", b)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	t.Setenv("WABOT_INBOUND_URL", srv.URL)
	t.Setenv("WABOT_INBOUND_TOKEN", "secret")
	t.Setenv("WABOT_INBOUND_TIMEOUT_SEC", "3")

	info := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:    types.JID{User: "111", Server: types.DefaultUserServer},
			Sender:  types.JID{User: "222", Server: types.DefaultUserServer},
			IsGroup: false,
		},
		ID:        types.MessageID("abc123"),
		Timestamp: time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC),
		PushName:  "Bob",
	}
	postInboundWebhook(info, "hello")

	if gotAuth != "Bearer secret" {
		t.Fatalf("Authorization: %q", gotAuth)
	}
}

func TestPostInboundWebhookNoURL(t *testing.T) {
	t.Setenv("WABOT_INBOUND_URL", "")
	postInboundWebhook(types.MessageInfo{}, "x")
}

func TestScheduleReconnectNilClient(t *testing.T) {
	old := client
	client = nil
	t.Cleanup(func() { client = old })
	scheduleReconnect("test")
}

func TestLoggedOutExits(t *testing.T) {
	var exitCode int
	old := osExit
	osExit = func(code int) { exitCode = code }
	defer func() { osExit = old }()

	handleConnectionEvents(&events.LoggedOut{
		OnConnect: false,
		Reason:    events.ConnectFailureLoggedOut,
	})
	if exitCode != 1 {
		t.Fatalf("exit code %d", exitCode)
	}
}

func TestDisconnectLogThrottled(t *testing.T) {
	lastDisconnectLog = time.Time{}
	logDisconnectThrottled()
	logDisconnectThrottled()
	logDisconnectThrottled()
}

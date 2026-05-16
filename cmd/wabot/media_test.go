package main

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestExtractMediaMeta(t *testing.T) {
	meta := extractMediaMeta(&waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			FileName: proto.String("report.pdf"),
			Mimetype: proto.String("application/pdf"),
		},
	})
	if meta.Kind != "document" || !meta.HasMedia || meta.Filename != "report.pdf" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
}

func TestMediaCacheRoundTrip(t *testing.T) {
	mediaCacheMu.Lock()
	mediaCache = make(map[mediaCacheKey][]byte, mediaCacheMax)
	mediaCacheQ = nil
	mediaCacheMu.Unlock()

	info := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:   types.JID{User: "111", Server: types.DefaultUserServer},
			Sender: types.JID{User: "222", Server: types.DefaultUserServer},
		},
		ID:        types.MessageID("msg-1"),
		Timestamp: time.Now().UTC(),
	}
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{Mimetype: proto.String("image/png")},
	}
	mediaCachePut(info, msg)
	got, ok := mediaCacheGet(info.Chat.String(), string(info.ID))
	if !ok || got.ImageMessage == nil {
		t.Fatalf("cache miss or wrong message: ok=%v", ok)
	}
}

func TestValidateSendMediaKind(t *testing.T) {
	if err := validateSendMediaKind("document", "application/pdf"); err != nil {
		t.Fatal(err)
	}
	if err := validateSendMediaKind("audio", "text/plain"); err == nil {
		t.Fatal("expected audio mime rejection")
	}
}

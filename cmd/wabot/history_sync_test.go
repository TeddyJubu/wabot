package main

import (
	"testing"
)

func TestIncludeHistoryPayload(t *testing.T) {
	if !includeHistoryPayload("hi", false) {
		t.Fatal("expected text message")
	}
	if includeHistoryPayload("", false) {
		t.Fatal("expected empty skip")
	}
	if !includeHistoryPayload("", true) {
		t.Fatal("expected media without text")
	}
}

func TestChunkInboundPayloads(t *testing.T) {
	msgs := []inboundPayload{
		{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"},
	}
	chunks := chunkInboundPayloads(msgs, 2)
	if len(chunks) != 3 {
		t.Fatalf("chunks %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[2]) != 1 {
		t.Fatalf("sizes %d %d", len(chunks[0]), len(chunks[2]))
	}
}

func TestHistoryDBStore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/history.db"
	msgs := []inboundPayload{
		{
			ID:        "m1",
			From:      "15550001111@s.whatsapp.net",
			Chat:      "15550001111@s.whatsapp.net",
			Text:      "hello",
			Timestamp: "2026-01-01T00:00:00Z",
		},
	}
	if err := historyDBStore(path, "RECENT", msgs); err != nil {
		t.Fatal(err)
	}
	if err := historyDBStore(path, "RECENT", msgs); err != nil {
		t.Fatal(err)
	}
}

func TestHistorySyncLimits(t *testing.T) {
	t.Setenv("WABOT_HISTORY_BATCH_SIZE", "0")
	t.Setenv("WABOT_HISTORY_MAX_MESSAGES", "0")
	batch, max := historySyncLimits()
	if batch != 1 || max != 1 {
		t.Fatalf("batch=%d max=%d", batch, max)
	}
}

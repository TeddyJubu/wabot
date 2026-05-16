package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	_ "github.com/mattn/go-sqlite3"
)

// historyBatchPayload is POSTed to WABOT_HISTORY_URL in batches.
type historyBatchPayload struct {
	Type         string           `json:"type"`
	SyncType     string           `json:"sync_type"`
	ChunkOrder   uint32           `json:"chunk_order,omitempty"`
	Progress     uint32           `json:"progress,omitempty"`
	MessageCount int              `json:"message_count"`
	Messages     []inboundPayload `json:"messages"`
}

func historySyncLimits() (batchSize, maxMessages int) {
	batchSize = envInt("WABOT_HISTORY_BATCH_SIZE", 50)
	if batchSize < 1 {
		batchSize = 1
	}
	maxMessages = envInt("WABOT_HISTORY_MAX_MESSAGES", 500)
	if maxMessages < 1 {
		maxMessages = 1
	}
	return batchSize, maxMessages
}

func conversationChatJID(conv *waHistorySync.Conversation) (types.JID, error) {
	if conv == nil {
		return types.EmptyJID, fmt.Errorf("nil conversation")
	}
	raw := conv.GetID()
	if raw == "" {
		raw = conv.GetNewJID()
	}
	if raw == "" {
		return types.EmptyJID, fmt.Errorf("conversation has no id")
	}
	return types.ParseJID(raw)
}

func includeHistoryPayload(text string, hasMedia bool) bool {
	if hasMedia {
		return true
	}
	return text != ""
}

func parseHistorySyncInbound(v *events.HistorySync, maxMessages int) []inboundPayload {
	if v == nil || v.Data == nil || client == nil {
		return nil
	}
	out := make([]inboundPayload, 0, min(maxMessages, 64))
	for _, conv := range v.Data.GetConversations() {
		if len(out) >= maxMessages {
			break
		}
		chatJID, err := conversationChatJID(conv)
		if err != nil {
			continue
		}
		for _, histMsg := range conv.GetMessages() {
			if len(out) >= maxMessages {
				break
			}
			webMsg := histMsg.GetMessage()
			if webMsg == nil {
				continue
			}
			evt, err := client.ParseWebMessage(chatJID, webMsg)
			if err != nil {
				continue
			}
			text := extractInboundText(evt.Message)
			payload := inboundPayloadFromMessage(evt.Info, text, evt.Message)
			if payload.ID == "" {
				continue
			}
			if !includeHistoryPayload(payload.Text, payload.HasMedia) {
				continue
			}
			out = append(out, payload)
		}
	}
	return out
}

func chunkInboundPayloads(messages []inboundPayload, batchSize int) [][]inboundPayload {
	if len(messages) == 0 {
		return nil
	}
	if batchSize < 1 {
		batchSize = 1
	}
	chunks := make([][]inboundPayload, 0, (len(messages)+batchSize-1)/batchSize)
	for i := 0; i < len(messages); i += batchSize {
		end := i + batchSize
		if end > len(messages) {
			end = len(messages)
		}
		chunks = append(chunks, messages[i:end])
	}
	return chunks
}

func deliverHistorySyncMessages(v *events.HistorySync) {
	if v == nil || v.Data == nil {
		return
	}
	historyURL := os.Getenv("WABOT_HISTORY_URL")
	historyDB := os.Getenv("WABOT_HISTORY_DB")
	if historyURL == "" && historyDB == "" {
		return
	}

	batchSize, maxMessages := historySyncLimits()
	messages := parseHistorySyncInbound(v, maxMessages)
	if len(messages) == 0 {
		return
	}

	if historyDB != "" {
		if err := historyDBStore(historyDB, v.Data.GetSyncType().String(), messages); err != nil {
			fmt.Println("history db:", err)
		}
	}

	if historyURL == "" {
		return
	}

	syncType := v.Data.GetSyncType().String()
	chunk := v.Data.GetChunkOrder()
	progress := v.Data.GetProgress()
	for _, batch := range chunkInboundPayloads(messages, batchSize) {
		payload := historyBatchPayload{
			Type:         "history_batch",
			SyncType:     syncType,
			ChunkOrder:   chunk,
			Progress:     progress,
			MessageCount: len(batch),
			Messages:     batch,
		}
		postJSONWebhookURL(historyURL, payload)
	}
}

func postHistorySyncSummary(v *events.HistorySync) {
	if v == nil || v.Data == nil {
		return
	}
	if os.Getenv("WABOT_HISTORY_SYNC_URL") == "" {
		return
	}
	convs := len(v.Data.GetConversations())
	msgs := 0
	for _, c := range v.Data.GetConversations() {
		msgs += len(c.GetMessages())
	}
	payload := historySyncPayload{
		Type:              "history_sync",
		SyncType:          v.Data.GetSyncType().String(),
		ConversationCount: convs,
		MessageCount:      msgs,
		ChunkOrder:        v.Data.GetChunkOrder(),
		Progress:          v.Data.GetProgress(),
	}
	postJSONWebhook("WABOT_HISTORY_SYNC_URL", payload)
}

var (
	historyDBMu     sync.Mutex
	historyDB       *sql.DB
	historyDBPath   string
)

func historyDBConn(path string) (*sql.DB, error) {
	historyDBMu.Lock()
	defer historyDBMu.Unlock()
	if historyDB != nil && historyDBPath == path {
		return historyDB, nil
	}
	if historyDB != nil {
		_ = historyDB.Close()
		historyDB = nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		create table if not exists history_messages (
			message_id text primary key,
			chat text not null,
			sender text not null,
			text text not null,
			received_at text not null,
			sync_type text not null,
			payload_json text not null
		);
		create index if not exists idx_history_messages_chat on history_messages(chat);
	`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	historyDB = db
	historyDBPath = path
	return db, nil
}

func historyDBStore(path, syncType string, messages []inboundPayload) error {
	db, err := historyDBConn(path)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		insert into history_messages (message_id, chat, sender, text, received_at, sync_type, payload_json)
		values (?, ?, ?, ?, ?, ?, ?)
		on conflict(message_id) do update set
			chat = excluded.chat,
			sender = excluded.sender,
			text = excluded.text,
			received_at = excluded.received_at,
			sync_type = excluded.sync_type,
			payload_json = excluded.payload_json
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range messages {
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		_, err = stmt.Exec(m.ID, m.Chat, m.From, m.Text, m.Timestamp, syncType, string(raw))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

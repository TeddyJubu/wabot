package main

import (
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

func handleReceiptEvent(v *events.Receipt) {
	if v == nil {
		return
	}
	ids := make([]string, len(v.MessageIDs))
	for i, id := range v.MessageIDs {
		ids[i] = string(id)
	}
	payload := receiptPayload{
		Type:        "receipt",
		Chat:        v.Chat.String(),
		Sender:      v.Sender.String(),
		MessageIDs:  ids,
		ReceiptType: string(v.Type),
		Timestamp:   v.Timestamp.UTC().Format(time.RFC3339),
	}
	if !v.MessageSender.IsEmpty() {
		payload.MessageSender = v.MessageSender.String()
	}
	go postJSONWebhook("WABOT_RECEIPT_URL", payload)
}

func handleChatPresenceEvent(v *events.ChatPresence) {
	if v == nil {
		return
	}
	media := ""
	if v.Media != "" {
		media = string(v.Media)
	}
	payload := presencePayload{
		Type:   "chat_presence",
		Chat:   v.Chat.String(),
		Sender: v.Sender.String(),
		State:  string(v.State),
		Media:  media,
	}
	go postJSONWebhook("WABOT_PRESENCE_URL", payload)
}

func handleHistorySyncEvent(v *events.HistorySync) {
	if v == nil || v.Data == nil {
		return
	}
	go postHistorySyncSummary(v)
	go deliverHistorySyncMessages(v)
}

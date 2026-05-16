package main

// receiptPayload is POSTed to WABOT_RECEIPT_URL when read/delivery receipts arrive.
type receiptPayload struct {
	Type          string   `json:"type"`
	Chat          string   `json:"chat"`
	Sender        string   `json:"sender,omitempty"`
	MessageIDs    []string `json:"message_ids"`
	ReceiptType   string   `json:"receipt_type"`
	Timestamp     string   `json:"timestamp"`
	MessageSender string   `json:"message_sender,omitempty"`
}

// presencePayload is POSTed to WABOT_PRESENCE_URL for remote typing indicators.
type presencePayload struct {
	Type   string `json:"type"`
	Chat   string `json:"chat"`
	Sender string `json:"sender"`
	State  string `json:"state"`
	Media  string `json:"media,omitempty"`
}

// historySyncPayload is a compact summary for optional WABOT_HISTORY_SYNC_URL.
type historySyncPayload struct {
	Type              string `json:"type"`
	SyncType          string `json:"sync_type"`
	ConversationCount int    `json:"conversation_count"`
	MessageCount      int    `json:"message_count"`
}

package main

// inboundPayload is the JSON shape for inbox, webhooks, and agent inbound API.
type inboundPayload struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	From      string `json:"from"`
	Chat      string `json:"chat"`
	IsGroup   bool   `json:"is_group"`
	PushName  string `json:"push_name,omitempty"`
	Text      string `json:"text"`
	MediaKind string `json:"media_kind,omitempty"`
	MediaMime string `json:"media_mime,omitempty"`
	MediaName string `json:"media_filename,omitempty"`
	HasMedia  bool   `json:"has_media,omitempty"`
}

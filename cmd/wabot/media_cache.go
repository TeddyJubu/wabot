package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const mediaCacheMax = 200

type mediaCacheKey struct {
	chat string
	id   string
}

var (
	mediaCacheMu sync.Mutex
	mediaCache   = make(map[mediaCacheKey][]byte, mediaCacheMax)
	mediaCacheQ  []mediaCacheKey
)

func mediaCachePut(info types.MessageInfo, msg *waE2E.Message) {
	if msg == nil || !messageHasDownloadableMedia(msg) {
		return
	}
	raw, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	key := mediaCacheKey{chat: info.Chat.String(), id: string(info.ID)}
	mediaCacheMu.Lock()
	defer mediaCacheMu.Unlock()
	if _, exists := mediaCache[key]; !exists {
		mediaCacheQ = append(mediaCacheQ, key)
		if len(mediaCacheQ) > mediaCacheMax {
			old := mediaCacheQ[0]
			mediaCacheQ = mediaCacheQ[1:]
			delete(mediaCache, old)
		}
	}
	mediaCache[key] = raw
}

func mediaCacheGet(chat, id string) (*waE2E.Message, bool) {
	key := mediaCacheKey{chat: chat, id: id}
	mediaCacheMu.Lock()
	raw, ok := mediaCache[key]
	mediaCacheMu.Unlock()
	if !ok {
		return nil, false
	}
	out := &waE2E.Message{}
	if err := proto.Unmarshal(raw, out); err != nil {
		return nil, false
	}
	return out, true
}

func messageHasDownloadableMedia(m *waE2E.Message) bool {
	if m == nil {
		return false
	}
	return m.ImageMessage != nil || m.VideoMessage != nil || m.AudioMessage != nil ||
		m.DocumentMessage != nil || m.StickerMessage != nil
}

type mediaMeta struct {
	Kind     string
	Mime     string
	Filename string
	HasMedia bool
}

func extractMediaMeta(m *waE2E.Message) mediaMeta {
	if m == nil {
		return mediaMeta{}
	}
	switch {
	case m.ImageMessage != nil:
		img := m.ImageMessage
		return mediaMeta{
			Kind:     "image",
			Mime:     img.GetMimetype(),
			Filename: "",
			HasMedia: true,
		}
	case m.VideoMessage != nil:
		vid := m.VideoMessage
		return mediaMeta{
			Kind:     "video",
			Mime:     vid.GetMimetype(),
			Filename: "",
			HasMedia: true,
		}
	case m.AudioMessage != nil:
		aud := m.AudioMessage
		return mediaMeta{
			Kind:     "audio",
			Mime:     aud.GetMimetype(),
			Filename: "",
			HasMedia: true,
		}
	case m.DocumentMessage != nil:
		doc := m.DocumentMessage
		return mediaMeta{
			Kind:     "document",
			Mime:     doc.GetMimetype(),
			Filename: doc.GetFileName(),
			HasMedia: true,
		}
	case m.StickerMessage != nil:
		st := m.StickerMessage
		return mediaMeta{
			Kind:     "sticker",
			Mime:     st.GetMimetype(),
			Filename: "",
			HasMedia: true,
		}
	default:
		return mediaMeta{}
	}
}

func inboundPayloadFromMessage(info types.MessageInfo, text string, msg *waE2E.Message) inboundPayload {
	meta := extractMediaMeta(msg)
	return inboundPayload{
		ID:        string(info.ID),
		Timestamp: info.Timestamp.UTC().Format(time.RFC3339Nano),
		From:      info.Sender.String(),
		Chat:      info.Chat.String(),
		IsGroup:   info.IsGroup,
		PushName:  info.PushName,
		Text:      text,
		MediaKind: meta.Kind,
		MediaMime: meta.Mime,
		MediaName: meta.Filename,
		HasMedia:  meta.HasMedia,
	}
}

func mediaKindDefaultMime(kind string) string {
	switch kind {
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "audio":
		return "audio/ogg; codecs=opus"
	case "document":
		return "application/octet-stream"
	case "sticker":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func mediaDownloadFilename(kind, mime, suggested string) string {
	if suggested != "" {
		return suggested
	}
	ext := ".bin"
	switch kind {
	case "image":
		ext = ".jpg"
	case "video":
		ext = ".mp4"
	case "audio":
		ext = ".ogg"
	case "document":
		ext = ".bin"
	case "sticker":
		ext = ".webp"
	}
	if mime != "" {
		if strings.HasPrefix(mime, "image/png") {
			ext = ".png"
		} else if strings.HasPrefix(mime, "image/webp") {
			ext = ".webp"
		} else if strings.HasPrefix(mime, "video/") {
			ext = ".mp4"
		} else if strings.HasPrefix(mime, "audio/") {
			ext = ".ogg"
		} else if strings.HasPrefix(mime, "application/pdf") {
			ext = ".pdf"
		}
	}
	return fmt.Sprintf("whatsapp-%s%s", kind, ext)
}

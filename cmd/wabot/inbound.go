package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// extractInboundText returns user-visible text for webhook + auto-reply routing.
func extractInboundText(m *waE2E.Message) string {
	if m == nil {
		return ""
	}
	if t := m.GetConversation(); t != "" {
		return t
	}
	if et := m.GetExtendedTextMessage(); et != nil {
		if t := et.GetText(); t != "" {
			return t
		}
	}
	if img := m.GetImageMessage(); img != nil {
		if c := img.GetCaption(); c != "" {
			return c
		}
		return "[image]"
	}
	if vid := m.GetVideoMessage(); vid != nil {
		if c := vid.GetCaption(); c != "" {
			return c
		}
		return "[video]"
	}
	if doc := m.GetDocumentMessage(); doc != nil {
		if c := doc.GetCaption(); c != "" {
			return c
		}
		if fn := doc.GetFileName(); fn != "" {
			return "[document: " + fn + "]"
		}
		return "[document]"
	}
	if m.GetStickerMessage() != nil {
		return "[sticker]"
	}
	if m.GetAudioMessage() != nil {
		return "[audio]"
	}
	if m.GetContactMessage() != nil {
		return "[contact]"
	}
	if loc := m.GetLocationMessage(); loc != nil {
		return fmt.Sprintf("[location %.4f,%.4f]", loc.GetDegreesLatitude(), loc.GetDegreesLongitude())
	}
	return ""
}

func postInboundWebhook(info types.MessageInfo, text string, msg *waE2E.Message) {
	url := os.Getenv("WABOT_INBOUND_URL")
	if url == "" {
		return
	}
	sec := envInt("WABOT_INBOUND_TIMEOUT_SEC", 10)
	if sec < 1 {
		sec = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
	defer cancel()

	payload := inboundPayloadFromMessage(info, text, msg)
	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("inbound webhook: marshal:", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Println("inbound webhook: request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := os.Getenv("WABOT_INBOUND_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("inbound webhook:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("inbound webhook: upstream returned HTTP", resp.StatusCode)
	}
}

func handleIncomingMessage(v *events.Message) {
	text := extractInboundText(v.Message)
	mediaCachePut(v.Info, v.Message)
	if v.Info.IsFromMe {
		recordInboundMessage(v.Info, text, v.Message)
		return
	}

	fmt.Println("From", v.Info.Sender.User, ":", text)
	recordInboundMessage(v.Info, text, v.Message)

	if os.Getenv("WABOT_INBOUND_URL") != "" {
		go postInboundWebhook(v.Info, text, v.Message)
	}

	var reply string
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "ping":
		reply = "pong 🏓"
	case "hi", "hello":
		reply = "Hey there! I'm a bot 🤖"
	case "time":
		reply = "It's " + time.Now().Format("3:04 PM") + " on my server"
	default:
		return
	}

	if client == nil {
		return
	}
	_, err := client.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		Conversation: proto.String(reply),
	})
	if err != nil {
		fmt.Println("Failed to send reply:", err)
	}
}

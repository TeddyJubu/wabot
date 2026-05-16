package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

const maxMediaBytes = 64 << 20 // 64 MB

func handleMediaDownload(w http.ResponseWriter, r *http.Request) {
	chat := strings.TrimSpace(r.URL.Query().Get("chat"))
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if chat == "" || id == "" {
		http.Error(w, "missing 'chat' or 'id' query parameter", http.StatusBadRequest)
		return
	}
	if !requireReadyClient(w) {
		return
	}

	msg, ok := mediaCacheGet(chat, id)
	if !ok {
		http.Error(w, "message not in media cache (only recent inbound media is available)", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	data, err := client.DownloadAny(ctx, msg)
	if err != nil {
		http.Error(w, "download: "+err.Error(), http.StatusBadGateway)
		return
	}

	meta := extractMediaMeta(msg)
	mime := meta.Mime
	if mime == "" {
		mime = mediaKindDefaultMime(meta.Kind)
	}
	filename := mediaDownloadFilename(meta.Kind, mime, meta.Filename)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Media-Kind", meta.Kind)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func handleSendMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaBytes+1<<20)
	if err := r.ParseMultipartForm(maxMediaBytes + 1<<20); err != nil {
		http.Error(w, "invalid multipart: "+err.Error(), http.StatusBadRequest)
		return
	}

	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	to := strings.TrimSpace(r.FormValue("to"))
	caption := r.FormValue("caption")
	filename := r.FormValue("filename")
	if kind == "" || to == "" {
		http.Error(w, "missing 'kind' or 'to'", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file': "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMediaBytes+1))
	if err != nil {
		http.Error(w, "read file: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}
	if len(data) > maxMediaBytes {
		http.Error(w, fmt.Sprintf("file too large (>%d bytes)", maxMediaBytes), http.StatusRequestEntityTooLarge)
		return
	}
	if filename == "" && header != nil {
		filename = header.Filename
	}

	mime := http.DetectContentType(data)
	if err := validateSendMediaKind(kind, mime); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	jid, err := resolveJID(to)
	if err != nil {
		http.Error(w, "bad recipient: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !client.IsLoggedIn() || !client.IsConnected() {
		http.Error(w, "whatsapp client not ready", http.StatusServiceUnavailable)
		return
	}
	if !reserveSend(w) {
		return
	}

	sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	msg, mediaType, err := buildOutboundMediaMessage(sendCtx, kind, data, mime, caption, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	resp, sendErr := client.SendMessage(sendCtx, jid, msg)
	entry := map[string]any{
		"kind":     kind,
		"to":       jid.String(),
		"caption":  caption,
		"bytes":    len(data),
		"mime":     mime,
		"filename": filename,
	}
	if sendErr != nil {
		entry["err"] = sendErr.Error()
		sends.write(entry)
		http.Error(w, sendErr.Error(), http.StatusInternalServerError)
		return
	}
	entry["id"] = resp.ID
	entry["media_type"] = string(mediaType)
	entry["timestamp"] = resp.Timestamp.UTC().Format(time.RFC3339)
	sends.write(entry)

	writeJSON(w, map[string]any{
		"id":        resp.ID,
		"timestamp": resp.Timestamp,
		"to":        jid.String(),
		"kind":      kind,
		"mime":      mime,
		"bytes":     len(data),
	})
}

func validateSendMediaKind(kind, mime string) error {
	switch kind {
	case "document":
		return nil
	case "audio":
		if !strings.HasPrefix(mime, "audio/") && mime != "application/octet-stream" {
			return fmt.Errorf("file is not audio (detected %s)", mime)
		}
	case "video":
		if !strings.HasPrefix(mime, "video/") && mime != "application/octet-stream" {
			return fmt.Errorf("file is not video (detected %s)", mime)
		}
	default:
		return fmt.Errorf("unsupported kind %q (use document, audio, or video)", kind)
	}
	return nil
}

func buildOutboundMediaMessage(
	ctx context.Context,
	kind string,
	data []byte,
	mime, caption, filename string,
) (*waE2E.Message, whatsmeow.MediaType, error) {
	var mediaType whatsmeow.MediaType
	switch kind {
	case "document":
		mediaType = whatsmeow.MediaDocument
	case "audio":
		mediaType = whatsmeow.MediaAudio
	case "video":
		mediaType = whatsmeow.MediaVideo
	default:
		return nil, "", fmt.Errorf("unsupported kind %q", kind)
	}

	uploaded, err := client.Upload(ctx, data, mediaType)
	if err != nil {
		return nil, "", fmt.Errorf("upload: %w", err)
	}

	switch kind {
	case "document":
		doc := &waE2E.DocumentMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			Mimetype:      proto.String(mime),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
		}
		if filename != "" {
			doc.FileName = proto.String(filename)
		}
		if caption != "" {
			doc.Caption = proto.String(caption)
		}
		return &waE2E.Message{DocumentMessage: doc}, mediaType, nil
	case "audio":
		aud := &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			Mimetype:      proto.String(mime),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			PTT:           proto.Bool(true),
		}
		return &waE2E.Message{AudioMessage: aud}, mediaType, nil
	case "video":
		vid := &waE2E.VideoMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			Mimetype:      proto.String(mime),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
		}
		if caption != "" {
			vid.Caption = proto.String(caption)
		}
		return &waE2E.Message{VideoMessage: vid}, mediaType, nil
	default:
		return nil, "", fmt.Errorf("unsupported kind %q", kind)
	}
}

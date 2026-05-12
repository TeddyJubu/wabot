package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
)

const (
	maxImageBytes   = 16 << 20 // 16 MB
	defaultLogPath  = "sends.log"
	defaultRatePerM = 20
	defaultBurst    = 5
	defaultHTTPAddr = "127.0.0.1:7777"
)

var (
	client      *whatsmeow.Client
	token       string
	sendLimiter *rate.Limiter
	sends       *sendLogger
)

type sendLogger struct {
	mu sync.Mutex
	f  *os.File
}

func openSendLogger(path string) (*sendLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &sendLogger{f: f}, nil
}

func (s *sendLogger) write(entry map[string]any) {
	if s == nil {
		return
	}
	entry["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	b = append(b, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.f.Write(b)
}

func (s *sendLogger) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.f.Close()
}

func eventHandler(evt interface{}) {
	handleConnectionEvents(evt)

	switch v := evt.(type) {
	case *events.Message:
		handleIncomingMessage(v)
	}
}

func resolveJID(to string) (types.JID, error) {
	if strings.ContainsAny(to, "@") {
		jid, err := types.ParseJID(to)
		if err != nil {
			return types.JID{}, err
		}
		return jid, nil
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, to)
	if digits == "" {
		return types.JID{}, fmt.Errorf("recipient %q has no digits", to)
	}
	return types.JID{User: digits, Server: types.DefaultUserServer}, nil
}

func authed(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("X-Token"))
		want := []byte(token)
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

// reserveSend tries to consume one token from the send rate limiter. Returns
// true when the caller may proceed. When the bucket is empty it writes a 429
// response (with Retry-After) and returns false. Crucially, callers should
// invoke this only *after* all input validation succeeds, so rejected requests
// don't burn budget.
func reserveSend(w http.ResponseWriter) bool {
	if sendLimiter == nil {
		return true
	}
	res := sendLimiter.Reserve()
	if !res.OK() || res.Delay() > 0 {
		delay := time.Second
		if res.OK() {
			delay = res.Delay()
			res.Cancel()
		}
		secs := int(delay.Round(time.Second).Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return false
	}
	return true
}

// sendTimeout bounds an individual whatsmeow call. It is intentionally not
// derived from r.Context() so a CLI hangup doesn't abort an in-flight send.
const sendTimeout = 60 * time.Second

func requireMethod(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.To == "" || req.Text == "" {
		http.Error(w, "missing 'to' or 'text'", http.StatusBadRequest)
		return
	}

	jid, err := resolveJID(req.To)
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
	resp, sendErr := client.SendMessage(sendCtx, jid, &waE2E.Message{
		Conversation: proto.String(req.Text),
	})
	entry := map[string]any{
		"kind": "text",
		"to":   jid.String(),
		"text": req.Text,
	}
	if sendErr != nil {
		entry["err"] = sendErr.Error()
		sends.write(entry)
		http.Error(w, sendErr.Error(), http.StatusInternalServerError)
		return
	}
	entry["id"] = resp.ID
	entry["timestamp"] = resp.Timestamp.UTC().Format(time.RFC3339)
	sends.write(entry)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":        resp.ID,
		"timestamp": resp.Timestamp,
		"to":        jid.String(),
	})
}

func handleSendImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+1<<20)
	if err := r.ParseMultipartForm(maxImageBytes + 1<<20); err != nil {
		http.Error(w, "invalid multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	to := r.FormValue("to")
	caption := r.FormValue("caption")
	if to == "" {
		http.Error(w, "missing 'to'", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing 'file': "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		http.Error(w, "read file: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}
	if len(data) > maxImageBytes {
		http.Error(w, fmt.Sprintf("file too large (>%d bytes)", maxImageBytes), http.StatusRequestEntityTooLarge)
		return
	}

	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/") {
		http.Error(w, "file is not an image (detected "+mime+")", http.StatusBadRequest)
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
	uploaded, err := client.Upload(sendCtx, data, whatsmeow.MediaImage)
	if err != nil {
		entry := map[string]any{
			"kind":     "image",
			"to":       jid.String(),
			"caption":  caption,
			"bytes":    len(data),
			"mime":     mime,
			"filename": header.Filename,
			"err":      "upload: " + err.Error(),
		}
		sends.write(entry)
		http.Error(w, "upload: "+err.Error(), http.StatusBadGateway)
		return
	}

	var width, height *uint32
	if cfg, _, dErr := image.DecodeConfig(bytes.NewReader(data)); dErr == nil {
		w32 := uint32(cfg.Width)
		h32 := uint32(cfg.Height)
		width = &w32
		height = &h32
	}

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			Mimetype:      proto.String(mime),
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uploaded.FileLength),
			Width:         width,
			Height:        height,
		},
	}
	if caption != "" {
		msg.ImageMessage.Caption = proto.String(caption)
	}

	resp, sendErr := client.SendMessage(sendCtx, jid, msg)
	entry := map[string]any{
		"kind":     "image",
		"to":       jid.String(),
		"caption":  caption,
		"bytes":    len(data),
		"mime":     mime,
		"filename": header.Filename,
	}
	if sendErr != nil {
		entry["err"] = sendErr.Error()
		sends.write(entry)
		http.Error(w, sendErr.Error(), http.StatusInternalServerError)
		return
	}
	entry["id"] = resp.ID
	entry["timestamp"] = resp.Timestamp.UTC().Format(time.RFC3339)
	sends.write(entry)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":        resp.ID,
		"timestamp": resp.Timestamp,
		"to":        jid.String(),
		"mime":      mime,
		"bytes":     len(data),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"logged_in": client != nil && client.IsLoggedIn(),
		"connected": client != nil && client.IsConnected(),
	})
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid %s=%q, using default %d\n", key, v, def)
		return def
	}
	return n
}

func main() {
	token = os.Getenv("WABOT_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "WABOT_TOKEN is not set; refusing to start (would allow unauthenticated sends)")
		os.Exit(1)
	}

	perMin := envInt("WABOT_RATE_PER_MIN", defaultRatePerM)
	burst := envInt("WABOT_RATE_BURST", defaultBurst)
	if perMin <= 0 || burst <= 0 {
		fmt.Println("Rate limiting disabled (set WABOT_RATE_PER_MIN>0 and WABOT_RATE_BURST>0 to enable)")
		sendLimiter = nil
	} else {
		sendLimiter = rate.NewLimiter(rate.Limit(float64(perMin)/60.0), burst)
		fmt.Printf("Rate limit: %d msg/min, burst %d\n", perMin, burst)
	}

	logPath := os.Getenv("WABOT_SEND_LOG")
	if logPath == "" {
		logPath = defaultLogPath
	}
	var err error
	sends, err = openSendLogger(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open send log %q: %v\n", logPath, err)
		os.Exit(1)
	}
	defer sends.close()
	fmt.Println("Send log:", logPath)

	httpAddr := os.Getenv("WABOT_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client = whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		qrChan, _ := client.GetQRChannel(context.Background())
		if err := client.Connect(); err != nil {
			fmt.Fprintln(os.Stderr, "connect:", err)
			os.Exit(1)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		if err := client.Connect(); err != nil {
			fmt.Fprintln(os.Stderr, "connect:", err)
			os.Exit(1)
		}
	}

	mux := http.NewServeMux()
	// Middleware order matters: auth first (free), then method check (free),
	// then the handler. Rate limit is consumed inside each handler *after* all
	// validation, so rejected requests never burn budget.
	mux.HandleFunc("/send", authed(requireMethod(http.MethodPost, handleSend)))
	mux.HandleFunc("/send-image", authed(requireMethod(http.MethodPost, handleSendImage)))
	mux.HandleFunc("/health", handleHealth)

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		fmt.Println("HTTP API listening on", httpAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	client.Disconnect()
}

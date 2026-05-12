package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultEndpoint = "http://127.0.0.1:7777"
)

var envOrder = []string{
	"WABOT_TOKEN",
	"WABOT_HTTP_ADDR",
	"WABOT_SEND_LOG",
	"WABOT_RATE_PER_MIN",
	"WABOT_RATE_BURST",
	"WABOT_INBOUND_URL",
	"WABOT_INBOUND_TOKEN",
	"WABOT_INBOUND_TIMEOUT_SEC",
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "send":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: wa send <number> <message>")
			os.Exit(2)
		}
		if len(os.Args) > 4 {
			fmt.Fprintln(os.Stderr, "too many arguments - did you forget to quote the message?")
			fmt.Fprintln(os.Stderr, "  e.g.  wa send 8801712345678 \"hello world\"")
			os.Exit(2)
		}
		send(os.Args[2], os.Args[3])
	case "send-image":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: wa send-image <number> <path> [caption]")
			os.Exit(2)
		}
		if len(os.Args) > 5 {
			fmt.Fprintln(os.Stderr, "too many arguments - did you forget to quote the caption?")
			fmt.Fprintln(os.Stderr, "  e.g.  wa send-image 880... ./pic.png \"build status\"")
			os.Exit(2)
		}
		caption := ""
		if len(os.Args) == 5 {
			caption = os.Args[4]
		}
		sendImage(os.Args[2], os.Args[3], caption)
	case "health":
		health()
	case "setup":
		setup()
	case "doctor":
		doctor()
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `wa - WhatsApp CLI for wabot

Quick start:
  wa setup
  wa health
  wa send 8801712345678 "hello"

Commands:
  wa setup                                    Interactive bootstrap (recommended)
  wa doctor                                   Check and auto-fix common local issues
  wa send <number> <message>                  Send a text message
  wa send-image <number> <path> [caption]     Send an image with optional caption
  wa health                                   Check whether the daemon is connected
  wa help                                     Show this help

Number format: country code + number, no '+' or spaces. Group JIDs
(123456-789@g.us) are accepted as-is.

Environment:
  WABOT_ENDPOINT    override daemon URL (default http://127.0.0.1:7777)
  WABOT_TOKEN       explicit token (optional if token file exists)
  WABOT_TOKEN_FILE  token file path override (default ~/.config/wabot/token)`)
	os.Exit(2)
}

func endpoint() string {
	if v := os.Getenv("WABOT_ENDPOINT"); v != "" {
		return v
	}
	return defaultEndpoint
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func tokenFilePath() string {
	if p := os.Getenv("WABOT_TOKEN_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".wabot-token"
	}
	return filepath.Join(home, ".config", "wabot", "token")
}

func readTokenFile() (string, error) {
	b, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return "", err
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", errors.New("token file is empty")
	}
	return t, nil
}

func writeTokenFile(token string) error {
	path := tokenFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func requireToken() string {
	if t := strings.TrimSpace(os.Getenv("WABOT_TOKEN")); t != "" {
		return t
	}
	if t, err := readTokenFile(); err == nil && t != "" {
		return t
	}
	fmt.Fprintln(os.Stderr, "error: token not found")
	fmt.Fprintln(os.Stderr, "run: wa setup")
	os.Exit(1)
	return ""
}

func writeResponse(resp *http.Response) {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "error (%d): %s\n", resp.StatusCode, bytes.TrimSpace(body))
		os.Exit(1)
	}
	os.Stdout.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		fmt.Println()
	}
}

func send(to, text string) {
	token := requireToken()
	body, err := json.Marshal(map[string]string{"to": to, "text": text})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: encoding request:", err)
		os.Exit(1)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint()+"/send", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	req.Header.Set("X-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient().Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	writeResponse(resp)
}

func sendImage(to, path, caption string) {
	token := requireToken()
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: open file:", err)
		os.Exit(1)
	}
	defer f.Close()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer mw.Close()
		if err := mw.WriteField("to", to); err != nil {
			pw.CloseWithError(err)
			return
		}
		if caption != "" {
			if err := mw.WriteField("caption", caption); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		fw, err := mw.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(fw, f); err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	req, err := http.NewRequest(http.MethodPost, endpoint()+"/send-image", pr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	req.Header.Set("X-Token", token)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := httpClient().Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	writeResponse(resp)
}

func health() {
	resp, err := httpClient().Get(endpoint() + "/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	os.Stdout.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		fmt.Println()
	}
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 10; i++ {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "scripts", "install.sh")) {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return "", errors.New("could not find repo root (go.mod + scripts/install.sh)")
}

func ask(reader *bufio.Reader, prompt, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", prompt, def)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func askBool(reader *bufio.Reader, prompt string, def bool) bool {
	suf := " [Y/n]: "
	if !def {
		suf = " [y/N]: "
	}
	fmt.Print(prompt + suf)
	line, _ := reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return line == "y" || line == "yes"
}

func loadEnvFile(path string) (map[string]string, error) {
	out := make(map[string]string)
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	lines := strings.Split(string(b), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, "=")
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	return out, nil
}

func writeEnvFile(path string, m map[string]string) error {
	var b strings.Builder
	b.WriteString("# Managed by wa setup / wa doctor.\n")
	seen := make(map[string]bool)
	for _, k := range envOrder {
		if v, ok := m[k]; ok && v != "" {
			b.WriteString(k + "=" + v + "\n")
			seen[k] = true
		}
	}
	var keys []string
	for k := range m {
		if !seen[k] && m[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(k + "=" + m[k] + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func runCmd(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func setup() {
	fmt.Println("wabot setup - interactive bootstrap")
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "run setup from inside the wabot repository")
		os.Exit(1)
	}
	reader := bufio.NewReader(os.Stdin)

	envPath := filepath.Join(root, "wabot.env")
	envMap := map[string]string{}
	if fileExists(envPath) {
		if m, err := loadEnvFile(envPath); err == nil {
			envMap = m
		}
	}

	token := strings.TrimSpace(envMap["WABOT_TOKEN"])
	if token == "" {
		if t, err := readTokenFile(); err == nil {
			token = t
		}
	}
	if token == "" {
		t, err := generateToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error generating token:", err)
			os.Exit(1)
		}
		token = t
		fmt.Println("- Generated a new local token.")
	}
	if err := writeTokenFile(token); err != nil {
		fmt.Fprintln(os.Stderr, "error writing token file:", err)
		os.Exit(1)
	}
	envMap["WABOT_TOKEN"] = token
	if _, ok := envMap["WABOT_HTTP_ADDR"]; !ok {
		envMap["WABOT_HTTP_ADDR"] = "127.0.0.1:7777"
	}
	if err := writeEnvFile(envPath, envMap); err != nil {
		fmt.Fprintln(os.Stderr, "error writing wabot.env:", err)
		os.Exit(1)
	}
	fmt.Printf("- Token saved at %s and synced to %s\n", tokenFilePath(), envPath)

	prefix := ask(reader, "Install binaries prefix", "/usr/local")
	doSystemd := askBool(reader, "Install and enable systemd services?", true)
	installDir := ask(reader, "Systemd INSTALL_DIR", root)
	user := ask(reader, "Systemd WABOT_USER", os.Getenv("USER"))

	if prefix != "" {
		fmt.Println("- Building and installing binaries...")
		if err := runCmd(root, nil, "bash", "./scripts/install.sh", "--prefix", prefix); err != nil {
			fmt.Fprintln(os.Stderr, "install failed:", err)
			os.Exit(1)
		}
	}

	if doSystemd {
		fmt.Println("- Installing systemd units...")
		env := []string{
			"INSTALL_DIR=" + installDir,
			"WABOT_USER=" + user,
		}
		if err := runCmd(root, env, "bash", "./scripts/install.sh", "--install-systemd"); err != nil {
			fmt.Fprintln(os.Stderr, "systemd install failed:", err)
			os.Exit(1)
		}
		_ = runCmd(root, nil, "sudo", "systemctl", "enable", "--now", "wabot.service")
		_ = runCmd(root, nil, "sudo", "systemctl", "enable", "--now", "wabot-backup-store.timer")
	}

	fmt.Println("- Running doctor...")
	doctor()
	fmt.Println("Setup complete.")
	fmt.Println("If this is first login, run: journalctl -u wabot -f   then scan the QR code.")
}

func doctor() {
	fmt.Println("wabot doctor - checks and quick fixes")
	root, _ := findRepoRoot()
	tokenPath := tokenFilePath()
	envPath := ""
	if root != "" {
		envPath = filepath.Join(root, "wabot.env")
	}

	var (
		tokenFileTok string
		tokenEnvTok  string
	)
	if t, err := readTokenFile(); err == nil {
		tokenFileTok = t
		fmt.Printf("[ok] token file present: %s\n", tokenPath)
	} else {
		fmt.Printf("[warn] token file missing/unreadable: %v\n", err)
	}

	if envPath != "" && fileExists(envPath) {
		if m, err := loadEnvFile(envPath); err == nil {
			tokenEnvTok = strings.TrimSpace(m["WABOT_TOKEN"])
			if tokenEnvTok != "" {
				fmt.Printf("[ok] wabot.env token found: %s\n", envPath)
			} else {
				fmt.Printf("[warn] WABOT_TOKEN missing in %s\n", envPath)
			}
		}
	}

	if tokenFileTok == "" && tokenEnvTok != "" {
		if err := writeTokenFile(tokenEnvTok); err == nil {
			tokenFileTok = tokenEnvTok
			fmt.Println("[fix] wrote token file from wabot.env")
		}
	}
	if tokenEnvTok == "" && tokenFileTok != "" && envPath != "" {
		m := map[string]string{}
		if fileExists(envPath) {
			m, _ = loadEnvFile(envPath)
		}
		m["WABOT_TOKEN"] = tokenFileTok
		if _, ok := m["WABOT_HTTP_ADDR"]; !ok {
			m["WABOT_HTTP_ADDR"] = "127.0.0.1:7777"
		}
		if err := writeEnvFile(envPath, m); err == nil {
			tokenEnvTok = tokenFileTok
			fmt.Println("[fix] wrote WABOT_TOKEN to wabot.env")
		}
	}
	if tokenFileTok != "" && tokenEnvTok != "" && tokenFileTok != tokenEnvTok {
		_ = writeTokenFile(tokenEnvTok)
		tokenFileTok = tokenEnvTok
		fmt.Println("[fix] token file and wabot.env differed; token file updated from wabot.env")
	}
	if fileExists(tokenPath) {
		_ = os.Chmod(tokenPath, 0o600)
	}

	// systemd status
	if _, err := exec.LookPath("systemctl"); err == nil {
		if err := exec.Command("systemctl", "is-active", "--quiet", "wabot").Run(); err != nil {
			fmt.Println("[warn] wabot.service is not active")
			if err := runCmd("", nil, "sudo", "systemctl", "start", "wabot"); err == nil {
				fmt.Println("[fix] started wabot.service")
			}
		} else {
			fmt.Println("[ok] wabot.service is active")
		}
	}

	// health check
	type healthResp struct {
		Connected bool `json:"connected"`
		LoggedIn  bool `json:"logged_in"`
	}
	var h healthResp
	resp, err := httpClient().Get(endpoint() + "/health")
	if err != nil {
		fmt.Printf("[warn] daemon health endpoint not reachable: %v\n", err)
		fmt.Println("       run `wa setup` or start wabot.service")
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[warn] /health returned HTTP %d\n", resp.StatusCode)
	} else if err := json.Unmarshal(b, &h); err == nil {
		fmt.Printf("[ok] daemon health: connected=%v logged_in=%v\n", h.Connected, h.LoggedIn)
		if !h.LoggedIn {
			fmt.Println("       not logged in yet - scan QR via `journalctl -u wabot -f`")
		}
	}

	// auth check (safe, no message send)
	tok := strings.TrimSpace(os.Getenv("WABOT_TOKEN"))
	if tok == "" {
		tok = tokenFileTok
	}
	if tok == "" {
		fmt.Println("[warn] no token available for auth probe (run `wa setup`)")
		return
	}
	body := bytes.NewBufferString(`{}`)
	req, _ := http.NewRequest(http.MethodPost, endpoint()+"/send", body)
	req.Header.Set("X-Token", tok)
	req.Header.Set("Content-Type", "application/json")
	r2, err := httpClient().Do(req)
	if err != nil {
		fmt.Printf("[warn] auth probe failed: %v\n", err)
		return
	}
	defer r2.Body.Close()
	if r2.StatusCode == http.StatusUnauthorized {
		fmt.Println("[warn] token rejected by daemon (401)")
		fmt.Println("       fix with `wa setup`")
		return
	}
	if r2.StatusCode == http.StatusBadRequest || r2.StatusCode == http.StatusTooManyRequests || r2.StatusCode == http.StatusServiceUnavailable {
		fmt.Println("[ok] token accepted by daemon")
		return
	}
	fmt.Printf("[warn] unexpected auth probe status: %d\n", r2.StatusCode)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

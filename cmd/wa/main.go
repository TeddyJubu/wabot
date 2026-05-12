package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const defaultEndpoint = "http://127.0.0.1:7777"

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
			fmt.Fprintln(os.Stderr, "too many arguments — did you forget to quote the message?")
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
			fmt.Fprintln(os.Stderr, "too many arguments — did you forget to quote the caption?")
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
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", os.Args[1])
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `wa — send WhatsApp messages from the command line

Commands:
  wa send <number> <message>                  Send a text message
  wa send-image <number> <path> [caption]     Send an image with optional caption
  wa health                                   Check whether the daemon is connected
  wa help                                     Show this help

Number format: country code + number, no '+' or spaces. Group JIDs
(123456-789@g.us) are accepted as-is.

Examples:
  wa send 8801712345678 "Hello from the CLI"
  wa send-image 8801712345678 ./screenshot.png "Build status"
  wa health

Environment:
  WABOT_TOKEN     shared secret with the daemon (required for send/send-image)
  WABOT_ENDPOINT  override daemon URL (default http://127.0.0.1:7777)`)
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

func requireToken() string {
	t := os.Getenv("WABOT_TOKEN")
	if t == "" {
		fmt.Fprintln(os.Stderr, "error: WABOT_TOKEN not set")
		os.Exit(1)
	}
	return t
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

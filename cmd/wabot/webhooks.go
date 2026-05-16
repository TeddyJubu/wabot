package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func webhookToken() string {
	if tok := os.Getenv("WABOT_INBOUND_TOKEN"); tok != "" {
		return tok
	}
	return os.Getenv("WABOT_WEBHOOK_TOKEN")
}

func postJSONWebhook(urlEnv string, payload any) {
	postJSONWebhookURL(os.Getenv(urlEnv), payload)
}

func postJSONWebhookURL(url string, payload any) {
	if url == "" {
		return
	}
	sec := envInt("WABOT_WEBHOOK_TIMEOUT_SEC", 10)
	if sec < 1 {
		sec = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
	defer cancel()

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("webhook:", url, "marshal:", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Println("webhook:", url, "request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := webhookToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("webhook:", url, ":", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("webhook:", url, "upstream HTTP", resp.StatusCode)
	}
}

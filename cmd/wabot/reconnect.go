package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// osExit is swappable in tests (avoid killing the test runner on LoggedOut).
var osExit = func(code int) { os.Exit(code) }

var reconnectBusy atomic.Bool

// scheduleReconnect handles *events.StreamReplaced and other cases where
// whatsmeow sets an "expected disconnect" flag, which suppresses the library's
// built-in autoReconnect path. We force a new session with ResetConnection +
// Connect and exponential backoff.
func scheduleReconnect(reason string) {
	c := client
	if c == nil {
		return
	}
	if !c.EnableAutoReconnect || c.Store == nil || c.Store.ID == nil {
		fmt.Printf("reconnect (%s): skipped (no session or autoreconnect off)\n", reason)
		return
	}
	if !reconnectBusy.CompareAndSwap(false, true) {
		fmt.Printf("reconnect (%s): already in progress\n", reason)
		return
	}
	go reconnectLoop(reason)
}

func reconnectLoop(reason string) {
	defer reconnectBusy.Store(false)

	const maxAttempts = 60
	backoff := time.Second
	maxBackoff := 60 * time.Second

	fmt.Printf("reconnect (%s): starting (up to %d attempts)\n", reason, maxAttempts)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Let the old socket finish tearing down after stream replace.
		if attempt == 1 && reason == "stream_replaced" {
			time.Sleep(2 * time.Second)
		} else {
			time.Sleep(backoff)
		}

		c := client
		if c == nil {
			return
		}
		if c.IsConnected() && c.IsLoggedIn() {
			fmt.Printf("reconnect (%s): session healthy after %d attempt(s)\n", reason, attempt)
			return
		}

		c.ResetConnection()
		time.Sleep(500 * time.Millisecond)

		err := c.Connect()
		if err == nil {
			fmt.Printf("reconnect (%s): Connect() ok on attempt %d\n", reason, attempt)
			return
		}
		if errors.Is(err, whatsmeow.ErrAlreadyConnected) && c.IsConnected() {
			fmt.Printf("reconnect (%s): already connected on attempt %d\n", reason, attempt)
			return
		}

		fmt.Printf("reconnect (%s): attempt %d/%d failed: %v\n", reason, attempt, maxAttempts, err)

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	fmt.Fprintf(os.Stderr, "reconnect (%s): exhausted attempts; exiting for systemd restart\n", reason)
	osExit(1)
}

var disconnectLogMu sync.Mutex
var lastDisconnectLog time.Time

func logDisconnectThrottled() {
	disconnectLogMu.Lock()
	defer disconnectLogMu.Unlock()
	now := time.Now()
	if now.Sub(lastDisconnectLog) < 5*time.Second {
		return
	}
	lastDisconnectLog = now
	fmt.Println("websocket disconnected (whatsmeow auto-reconnect should follow)")
}

func handleConnectionEvents(evt interface{}) {
	switch v := evt.(type) {
	case *events.StreamReplaced:
		fmt.Println("stream replaced (another session took over?) — forcing reconnect")
		scheduleReconnect("stream_replaced")

	case *events.LoggedOut:
		fmt.Fprintf(os.Stderr, "logged out / session invalidated: on_connect=%v reason=%v\n", v.OnConnect, v.Reason)
		osExit(1)

	case *events.TemporaryBan:
		fmt.Fprintf(os.Stderr, "temporary ban: %s\n", v.String())
		osExit(1)

	case *events.ClientOutdated:
		fmt.Fprintln(os.Stderr, "client outdated (405) — update go.mau.fi/whatsmeow")
		osExit(1)

	case *events.CATRefreshError:
		fmt.Fprintf(os.Stderr, "CAT refresh failed: %v\n", v.Error)
		osExit(1)

	case *events.ConnectFailure:
		fmt.Fprintf(os.Stderr, "connect failure: reason=%v message=%q\n", v.Reason, v.Message)

	case *events.Disconnected:
		logDisconnectThrottled()
	}
}

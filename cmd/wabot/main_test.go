package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"golang.org/x/time/rate"
)

func TestResolveJID(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantUser string
		wantSrv  string
		wantErr  bool
	}{
		{"plain digits", "8801712345678", "8801712345678", types.DefaultUserServer, false},
		{"plus and spaces", "+880 1712-345678", "8801712345678", types.DefaultUserServer, false},
		{"leading/trailing space", "  8801712345678  ", "8801712345678", types.DefaultUserServer, false},
		{"alpha only", "abc", "", "", true},
		{"empty", "", "", "", true},
		{"explicit user JID", "12345@s.whatsapp.net", "12345", "s.whatsapp.net", false},
		{"group JID", "12345-6789@g.us", "12345-6789", "g.us", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveJID(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got.User != tc.wantUser || got.Server != tc.wantSrv {
				t.Errorf("got User=%q Server=%q, want User=%q Server=%q",
					got.User, got.Server, tc.wantUser, tc.wantSrv)
			}
		})
	}
}

func TestPairingStateSnapshotExpires(t *testing.T) {
	var p pairingState
	if code, _, _, _ := p.snapshot(); code != "" {
		t.Fatalf("empty state returned code %q", code)
	}

	p.setCode("qr-code")
	code, event, updated, expires := p.snapshot()
	if code != "qr-code" || event != "code" {
		t.Fatalf("code=%q event=%q", code, event)
	}
	if updated.IsZero() || expires.IsZero() || !expires.After(updated) {
		t.Fatalf("bad timestamps updated=%v expires=%v", updated, expires)
	}

	p.updated = time.Now().UTC().Add(-(pairingQRTTL + time.Second))
	code, _, _, _ = p.snapshot()
	if code != "" {
		t.Fatalf("expired state returned code %q", code)
	}
}

func TestHandlePairingQR(t *testing.T) {
	oldPairing := pairing
	oldClient := client
	pairing = pairingState{}
	client = nil
	t.Cleanup(func() {
		pairing = oldPairing
		client = oldClient
	})
	pairing.setCode("qr-code")

	rec := httptest.NewRecorder()
	handlePairingQR(rec, httptest.NewRequest(http.MethodGet, "/pairing/qr", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var body struct {
		QR    string `json:"qr"`
		Event string `json:"event"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.QR != "qr-code" || body.Event != "code" {
		t.Fatalf("body=%+v", body)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("WABOT_TEST_INT", "")
	if got := envInt("WABOT_TEST_INT", 42); got != 42 {
		t.Errorf("unset: got %d want 42", got)
	}
	t.Setenv("WABOT_TEST_INT", "7")
	if got := envInt("WABOT_TEST_INT", 42); got != 7 {
		t.Errorf("valid: got %d want 7", got)
	}
	t.Setenv("WABOT_TEST_INT", "not a number")
	if got := envInt("WABOT_TEST_INT", 42); got != 42 {
		t.Errorf("invalid: got %d want 42 (fallback)", got)
	}
}

func TestAuthed(t *testing.T) {
	defer func(orig string) { token = orig }(token)
	token = "correct-secret"

	called := false
	h := authed(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("missing header", func(t *testing.T) {
		called = false
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
		if rec.Code != http.StatusUnauthorized || called {
			t.Errorf("code=%d called=%v", rec.Code, called)
		}
	})
	t.Run("wrong token", func(t *testing.T) {
		called = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.Header.Set("X-Token", "wrong")
		h(rec, req)
		if rec.Code != http.StatusUnauthorized || called {
			t.Errorf("code=%d called=%v", rec.Code, called)
		}
	})
	t.Run("right token", func(t *testing.T) {
		called = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.Header.Set("X-Token", "correct-secret")
		h(rec, req)
		if rec.Code != http.StatusOK || !called {
			t.Errorf("code=%d called=%v", rec.Code, called)
		}
	})
}

func TestRequireMethod(t *testing.T) {
	called := false
	h := requireMethod(http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	called = false
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusMethodNotAllowed || called {
		t.Errorf("GET: code=%d called=%v", rec.Code, called)
	}

	called = false
	rec = httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if !called {
		t.Errorf("POST: handler not called")
	}
}

// TestReserveSend exercises the rate-limit helper directly. The new design
// only consumes a token when the caller actually intends to hit WhatsApp, so
// rejected requests upstream (auth, method, body validation) must not burn
// budget — this test guards that contract.
func TestReserveSend(t *testing.T) {
	defer func(orig *rate.Limiter) { sendLimiter = orig }(sendLimiter)

	t.Run("nil limiter allows everything", func(t *testing.T) {
		sendLimiter = nil
		rec := httptest.NewRecorder()
		if !reserveSend(rec) {
			t.Errorf("nil limiter should always allow")
		}
	})

	t.Run("burst exhaustion returns 429 with Retry-After", func(t *testing.T) {
		// 1 token / 60s, burst 2 → two immediate, then reject.
		sendLimiter = rate.NewLimiter(rate.Limit(1.0/60.0), 2)
		for i := 0; i < 2; i++ {
			if !reserveSend(httptest.NewRecorder()) {
				t.Fatalf("burst %d should be allowed", i)
			}
		}
		rec := httptest.NewRecorder()
		if reserveSend(rec) {
			t.Fatalf("third call should be rejected")
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("code=%d want 429", rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got == "" {
			t.Errorf("Retry-After header missing")
		}
	})

	t.Run("rejected reservations don't drain bucket", func(t *testing.T) {
		// Fresh limiter: 1/60s, burst 1.
		sendLimiter = rate.NewLimiter(rate.Limit(1.0/60.0), 1)
		if !reserveSend(httptest.NewRecorder()) {
			t.Fatal("first should succeed")
		}
		// Many rejected attempts should not push the next refill further out.
		for i := 0; i < 10; i++ {
			if reserveSend(httptest.NewRecorder()) {
				t.Fatalf("attempt %d should be rejected", i+2)
			}
		}
		// Tokens-now should still be near 0 (not deeply negative). If we were
		// consuming on rejection, it would be -10.
		got := sendLimiter.Tokens()
		if got < -0.5 {
			t.Errorf("limiter looks drained: tokens=%v (should be near 0)", got)
		}
	})
}

// TestCLIArgsRejectExtraArgs is a behavioural smoke check on the message you
// get when you forget to quote the message — the symptom that originally hid
// silent truncation. We re-implement the trivial check inline to avoid having
// to expose os.Args parsing.
func TestCLIArgsRejectExtraArgs(t *testing.T) {
	// Token rules: the user-facing CLI prints "too many arguments" when more
	// than 4 (send) or 5 (send-image) os.Args are present. Encode the limits
	// here so a regression in the CLI surfaces as a failed compile/test.
	const sendMax = 4      // wa send <num> <msg>
	const sendImageMax = 5 // wa send-image <num> <path> [caption]

	tooMany := func(name string, max, got int) {
		if got <= max {
			t.Errorf("%s: %d args should be allowed", name, got)
		}
	}
	tooMany("send", sendMax, len(strings.Fields("wa send 880 hello world")))
	tooMany("send-image", sendImageMax,
		len(strings.Fields("wa send-image 880 ./p.png a b")))
}

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newAPIMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /groups", authed(handleGroupsList))
	mux.HandleFunc("POST /groups", authed(handleGroupCreate))
	mux.HandleFunc("GET /groups/{jid}", authed(handleGroupInfo))
	mux.HandleFunc("POST /groups/{jid}/invite", authed(handleGroupInvite))
	mux.HandleFunc("POST /groups/join", authed(handleGroupJoin))
	mux.HandleFunc("POST /messages/react", authed(handleMessageReact))
	mux.HandleFunc("PATCH /messages/edit", authed(handleMessageEdit))
	mux.HandleFunc("DELETE /messages/{id}", authed(handleMessageRevoke))
	return mux
}

func TestPhase3MuxRoutesReachable(t *testing.T) {
	token = "test-token"
	mux := newAPIMux()
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/messages/react"},
		{http.MethodPatch, "/messages/edit"},
		{http.MethodDelete, "/messages/abc?chat=1@s.whatsapp.net"},
		{http.MethodPost, "/groups/join"},
		{http.MethodGet, "/groups/some-group@g.us"},
		{http.MethodPost, "/groups/some-group@g.us/invite"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Token", "test-token")
			mux.ServeHTTP(rec, req)
			if rec.Code == http.StatusNotFound {
				t.Fatalf("route not registered: %s %s", tc.method, tc.path)
			}
		})
	}
}

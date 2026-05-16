package main

import "net/http"

// registerPhase3Routes wires Phase 3 REST handlers (Go 1.22+ method-prefixed patterns).
func registerPhase3Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /groups", authed(handleGroupsList))
	mux.HandleFunc("POST /groups", authed(handleGroupCreate))
	mux.HandleFunc("GET /groups/{jid}", authed(handleGroupInfo))
	mux.HandleFunc("POST /groups/{jid}/invite", authed(handleGroupInvite))
	mux.HandleFunc("POST /groups/join", authed(handleGroupJoin))
	mux.HandleFunc("POST /messages/react", authed(handleMessageReact))
	mux.HandleFunc("PATCH /messages/edit", authed(handleMessageEdit))
	mux.HandleFunc("DELETE /messages/{id}", authed(handleMessageRevoke))
}

// registerPhase4Routes wires receipt/presence webhooks (eventHandler) and chat app-state APIs.
func registerPhase4Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /chats/{jid}/mute", authed(handleChatMute))
	mux.HandleFunc("POST /chats/{jid}/archive", authed(handleChatArchive))
	mux.HandleFunc("POST /chats/{jid}/pin", authed(handleChatPin))
}

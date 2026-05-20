package main

import "net/http"

// registerPhase3Routes wires Phase 3 REST handlers (Go 1.22+ method-prefixed patterns).
func registerPhase3Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /groups", authed(handleGroupsList))
	mux.HandleFunc("POST /groups", authed(handleGroupCreate))
	mux.HandleFunc("GET /groups/{jid}", authed(handleGroupInfo))
	mux.HandleFunc("PATCH /groups/{jid}", authed(handleGroupUpdate))
	mux.HandleFunc("POST /groups/{jid}/participants", authed(handleGroupParticipants))
	mux.HandleFunc("POST /groups/{jid}/leave", authed(handleGroupLeave))
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

// registerPhase5Routes wires user profile info and avatar download.
func registerPhase5Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /users/{jid}", authed(handleUserInfo))
	mux.HandleFunc("GET /users/{jid}/picture", authed(handleUserPicture))
}

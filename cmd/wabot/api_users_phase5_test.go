package main

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestUserInfoToMap(t *testing.T) {
	jid := types.JID{User: "15550001111", Server: types.DefaultUserServer}
	m := userInfoToMap(jid, types.UserInfo{
		Status:    "Hey",
		PictureID: "abc123",
		LID:       types.JID{User: "99", Server: types.HiddenUserServer},
	})
	if m["jid"] != jid.String() {
		t.Fatalf("jid %v", m["jid"])
	}
	if m["status"] != "Hey" {
		t.Fatalf("status %v", m["status"])
	}
	if m["picture_id"] != "abc123" {
		t.Fatalf("picture_id %v", m["picture_id"])
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseMessageTargetRequiresFields(t *testing.T) {
	_, _, _, err := parseMessageTarget("", "", "id")
	if err == nil {
		t.Fatal("expected error for missing chat")
	}
}

func TestGroupInfoToMapEmpty(t *testing.T) {
	m := groupInfoToMap(nil)
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %+v", m)
	}
}

func TestHandleGroupsRootMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/groups", nil)
	handleGroupsRoot(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", rec.Code)
	}
}

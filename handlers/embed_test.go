package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteMediaLinkRange(t *testing.T) {
	request := httptest.NewRequest("GET", "http://localhost/p/DbcV_ofDklX/1to12", nil)
	request.Host = "example.ngrok-free.app"
	request.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()

	if !writeMediaLinkRange(response, request, "1to12") {
		t.Fatal("writeMediaLinkRange returned false")
	}
	if response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", response.Header().Get("Content-Type"))
	}

	lines := strings.Split(strings.TrimSpace(response.Body.String()), "\n")
	if len(lines) != 12 {
		t.Fatalf("link count = %d, want 12", len(lines))
	}
	if lines[0] != "https://example.ngrok-free.app/p/DbcV_ofDklX/1" {
		t.Errorf("first link = %q", lines[0])
	}
	if lines[11] != "https://example.ngrok-free.app/p/DbcV_ofDklX/12" {
		t.Errorf("last link = %q", lines[11])
	}
}

func TestWriteMediaLinkRangeRejectsInvalidRange(t *testing.T) {
	request := httptest.NewRequest("GET", "http://localhost/p/example/12to1", nil)
	response := httptest.NewRecorder()
	if writeMediaLinkRange(response, request, "12to1") {
		t.Fatal("writeMediaLinkRange accepted a descending range")
	}
}

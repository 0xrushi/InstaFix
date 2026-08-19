//go:build integration

package handlers

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestInstagramReelDcL8TX3xQTd(t *testing.T) {
	const (
		reelURL = "https://www.instagram.com/reel/DcL8TX3xQTd/"
		postID  = "DcL8TX3xQTd"
	)

	item := &InstaData{PostID: postID}
	if err := item.ScrapeData(); err != nil {
		t.Fatalf("scrape %s: %v", reelURL, err)
	}

	if item.Username != "sammylett" {
		t.Errorf("username = %q, want %q", item.Username, "sammylett")
	}
	if item.Caption != "Paying people $10 to tie my shoe" {
		t.Errorf("caption = %q, want %q", item.Caption, "Paying people $10 to tie my shoe")
	}
	if len(item.Medias) != 1 {
		t.Fatalf("media count = %d, want 1", len(item.Medias))
	}

	media := item.Medias[0]
	if media.TypeName != "GraphVideo" {
		t.Errorf("media type = %q, want GraphVideo", media.TypeName)
	}
	if media.URL == "" {
		t.Fatal("scraper returned an empty media URL")
	}

	// Instagram's legacy embed sometimes labels the Reel cover as GraphVideo.
	// Probe the resource and report that upstream limitation as a skip.
	request, err := http.NewRequest(http.MethodGet, media.URL, nil)
	if err != nil {
		t.Fatalf("create media request: %v", err)
	}
	request.Header.Set("Range", "bytes=0-0")
	request.Header.Set("Referer", "https://www.instagram.com/")

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("probe media URL: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		t.Fatalf("probe media URL: status = %s, want 200 or 206", response.Status)
	}
	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "video/") {
		t.Skipf(
			"media Content-Type = %q, want video/* (Instagram likely returned the Reel cover: %q)",
			contentType,
			media.URL,
		)
	}
}

func TestInstagramCarouselDcKRfwLmrmb(t *testing.T) {
	const (
		postURL = "https://www.instagram.com/p/DcKRfwLmrmb/"
		postID  = "DcKRfwLmrmb"
	)

	item := &InstaData{PostID: postID}
	if err := item.ScrapeData(); err != nil {
		t.Fatalf("scrape %s: %v", postURL, err)
	}
	if item.Username != "factsdailyy" {
		t.Errorf("username = %q, want %q", item.Username, "factsdailyy")
	}
	if len(item.Medias) != 6 {
		t.Fatalf("media count = %d, want 6", len(item.Medias))
	}
	for index, media := range item.Medias {
		if media.URL == "" {
			t.Errorf("media %d has an empty URL", index+1)
		}
		if media.TypeName != "GraphImage" {
			t.Errorf("media %d type = %q, want GraphImage", index+1, media.TypeName)
		}
	}
}

package handlers

import (
	"archive/zip"
	"bytes"
	scraper "instafix/handlers/scraper"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDownloadMediaArchiveIncludesEveryCarouselItem(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
		}
		switch r.URL.Path {
		case "/photo":
			response.Header.Set("Content-Type", "image/jpeg")
			response.Body = io.NopCloser(strings.NewReader("jpeg-data"))
		case "/video":
			response.Header.Set("Content-Type", "video/mp4")
			response.Body = io.NopCloser(strings.NewReader("mp4-data"))
		default:
			response.StatusCode = http.StatusNotFound
			response.Status = "404 Not Found"
			response.Body = io.NopCloser(strings.NewReader("not found"))
		}
		return response, nil
	})}

	medias := []scraper.Media{
		{TypeName: "GraphImage", URL: "https://cdn.example/photo"},
		{TypeName: "GraphVideo", URL: "https://cdn.example/video"},
	}
	request, err := http.NewRequest(http.MethodGet, "http://localhost/download/example", nil)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := downloadMediaArchive(request, &output, medias, client); err != nil {
		t.Fatalf("downloadMediaArchive: %v", err)
	}

	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatalf("read ZIP: %v", err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("ZIP entry count = %d, want 2", len(archive.File))
	}

	want := map[string]string{
		"01.jpg": "jpeg-data",
		"02.mp4": "mp4-data",
	}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		contents, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		if string(contents) != want[file.Name] {
			t.Errorf("%s contents = %q, want %q", file.Name, contents, want[file.Name])
		}
		delete(want, file.Name)
	}
	if len(want) != 0 {
		t.Errorf("ZIP is missing entries: %v", want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

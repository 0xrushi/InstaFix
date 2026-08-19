package handlers

import (
	"archive/zip"
	"errors"
	"fmt"
	scraper "instafix/handlers/scraper"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

var downloadClient = &http.Client{Timeout: 2 * time.Minute}

// Download packages every image and video in a post into one ZIP download.
func Download(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "postID")
	item, err := scraper.GetData(postID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(item.Medias) == 0 {
		http.Error(w, "post contains no downloadable media", http.StatusNotFound)
		return
	}

	archive, err := os.CreateTemp("", "instafix-*.zip")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	archiveName := archive.Name()
	defer os.Remove(archiveName)

	if err := downloadMediaArchive(r, archive, item.Medias, downloadClient); err != nil {
		archive.Close()
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := archive.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	archive, err = os.Open(archiveName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer archive.Close()
	info, err := archive.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": "instagram-" + postID + ".zip",
	})
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Type", "application/zip")
	http.ServeContent(w, r, info.Name(), info.ModTime(), archive)
}

func downloadMediaArchive(r *http.Request, dst io.Writer, medias []scraper.Media, client *http.Client) error {
	archive := zip.NewWriter(dst)
	for index, media := range medias {
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, media.URL, nil)
		if err != nil {
			return errors.Join(err, archive.Close())
		}
		request.Header.Set("Referer", "https://www.instagram.com/")
		request.Header.Set("User-Agent", "Mozilla/5.0")

		response, err := client.Do(request)
		if err != nil {
			return errors.Join(fmt.Errorf("download media %d: %w", index+1, err), archive.Close())
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return errors.Join(
				fmt.Errorf("download media %d: status %s", index+1, response.Status),
				archive.Close(),
			)
		}

		header := &zip.FileHeader{
			Name:   fmt.Sprintf("%02d%s", index+1, mediaExtension(response.Header.Get("Content-Type"), media)),
			Method: zip.Store,
		}
		entry, err := archive.CreateHeader(header)
		if err == nil {
			_, err = io.Copy(entry, response.Body)
		}
		closeErr := response.Body.Close()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr, archive.Close())
		}
	}
	return archive.Close()
}

func mediaExtension(contentType string, media scraper.Media) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	}

	if parsed, err := url.Parse(media.URL); err == nil {
		if extension := path.Ext(parsed.Path); extension != "" && len(extension) <= 5 {
			return strings.ToLower(extension)
		}
	}
	if strings.Contains(media.TypeName, "Video") {
		return ".mp4"
	}
	return ".jpg"
}

package handlers

import (
	"errors"
	"fmt"
	scraper "instafix/handlers/scraper"
	"instafix/utils"
	"instafix/views"
	"instafix/views/model"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func mediaidToCode(mediaID int) string {
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var shortCode string

	for mediaID > 0 {
		remainder := mediaID % 64
		mediaID /= 64
		shortCode = string(alphabet[remainder]) + shortCode
	}

	return shortCode
}

func getSharePostID(postID string) (string, error) {
	req, err := http.NewRequest("HEAD", "https://www.instagram.com/share/reel/"+postID+"/", nil)
	if err != nil {
		return postID, err
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return postID, err
	}
	defer resp.Body.Close()
	redirURL, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return postID, err
	}
	postID = path.Base(redirURL.Path)
	if postID == "login" {
		return postID, errors.New("not logged in")
	}
	return postID, nil
}

func Embed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	viewsData := &model.ViewsData{}

	var err error
	postID := chi.URLParam(r, "postID")
	mediaNumParams := chi.URLParam(r, "mediaNum")
	urlQuery := r.URL.Query()
	if urlQuery == nil {
		return
	}
	if mediaNumParams == "" {
		imgIndex := urlQuery.Get("img_index")
		if imgIndex != "" {
			mediaNumParams = imgIndex
		} else {
			mediaNumParams = "0"
		}
	}
	if writeMediaLinkRange(w, r, mediaNumParams) {
		return
	}
	mediaNum, err := strconv.Atoi(mediaNumParams)
	if err != nil {
		viewsData.Description = "Invalid img_index parameter"
		views.Embed(viewsData, w)
		return
	}

	isDirect, _ := strconv.ParseBool(urlQuery.Get("direct"))
	isGallery, _ := strconv.ParseBool(urlQuery.Get("gallery"))

	// Get direct/gallery from header too, nginx query params is pain in the ass
	embedType := r.Header.Get("X-Embed-Type")
	if embedType == "direct" {
		isDirect = true
	} else if embedType == "gallery" {
		isGallery = true
	}

	// Stories use mediaID (int) instead of postID
	if strings.Contains(r.URL.Path, "/stories/") {
		mediaID, err := strconv.Atoi(postID)
		if err != nil {
			viewsData.Description = "Invalid postID"
			views.Embed(viewsData, w)
			return
		}
		postID = mediaidToCode(mediaID)
	} else if strings.Contains(r.URL.Path, "/share/") {
		postID, err = getSharePostID(postID)
		if err != nil && len(scraper.RemoteScraperAddr) == 0 {
			slog.Error("Failed to get new postID from share URL", "postID", postID, "err", err)
			viewsData.Description = "Failed to get new postID from share URL"
			views.Embed(viewsData, w)
			return
		}
	}

	// If User-Agent is not bot, redirect to Instagram
	viewsData.Title = "InstaFix"
	viewsData.URL = "https://instagram.com" + strings.Replace(r.URL.RequestURI(), "/"+mediaNumParams, "", 1)
	if !utils.IsBot(r.Header.Get("User-Agent")) {
		http.Redirect(w, r, viewsData.URL, http.StatusFound)
		return
	}

	item, err := scraper.GetData(postID)
	if err != nil || len(item.Medias) == 0 {
		http.Redirect(w, r, viewsData.URL, http.StatusFound)
		return
	}

	if mediaNum > len(item.Medias) {
		viewsData.Description = "Media number out of range"
		views.Embed(viewsData, w)
		return
	} else if len(item.Username) == 0 {
		viewsData.Description = "Post not found"
		views.Embed(viewsData, w)
		return
	}

	var sb strings.Builder
	sb.Grow(32) // 32 bytes should be enough for most cases

	viewsData.Title = "@" + item.Username
	// Gallery do not have any caption
	if !isGallery {
		viewsData.Description = item.Caption
		if len(viewsData.Description) > 255 {
			viewsData.Description = utils.Substr(viewsData.Description, 0, 250) + "..."
		}
	}

	typename := item.Medias[max(1, mediaNum)-1].TypeName
	isImage := strings.Contains(typename, "Image") || strings.Contains(typename, "StoryVideo")
	switch {
	case isImage:
		viewsData.Card = "summary_large_image"
		sb.WriteString("/images/")
		sb.WriteString(postID)
		sb.WriteString("/")
		sb.WriteString(strconv.Itoa(max(1, mediaNum)))
		viewsData.ImageURL = sb.String()
	default:
		viewsData.Card = "player"
		sb.WriteString("/videos/")
		sb.WriteString(postID)
		sb.WriteString("/")
		sb.WriteString(strconv.Itoa(max(1, mediaNum)))
		viewsData.VideoURL = sb.String()

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		viewsData.OEmbedURL = scheme + "://" + r.Host + "/oembed?text=" + url.QueryEscape(viewsData.Description) + "&url=" + viewsData.URL
	}
	if isDirect {
		http.Redirect(w, r, sb.String(), http.StatusFound)
		return
	}

	views.Embed(viewsData, w)
}

func writeMediaLinkRange(w http.ResponseWriter, r *http.Request, value string) bool {
	startText, endText, ok := strings.Cut(value, "to")
	if !ok || startText == "" || endText == "" {
		return false
	}
	start, startErr := strconv.Atoi(startText)
	end, endErr := strconv.Atoi(endText)
	if startErr != nil || endErr != nil || start < 1 || end < start || end-start >= 100 {
		return false
	}

	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme != "http" && scheme != "https" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	basePath := strings.TrimSuffix(r.URL.Path, "/"+value)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for mediaNumber := start; mediaNumber <= end; mediaNumber++ {
		fmt.Fprintf(w, "%s://%s%s/%d\n", scheme, r.Host, basePath, mediaNumber)
	}
	return true
}

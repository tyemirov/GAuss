package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/gauss"
	"github.com/tyemirov/GAuss/pkg/session"
	"github.com/tyemirov/utils/system"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const (
	mainPagePath = "/youtube"
	Root         = "/"
	baseURL      = "http://localhost:8080/"
)

func main() {
	loginTemplateFlag := flag.String("template", "", "Path to custom login template (empty for default)")
	flag.Parse()

	clientSecret := system.GetEnvOrFail("SESSION_SECRET")
	googleClientID := system.GetEnvOrFail("GOOGLE_CLIENT_ID")
	googleClientSecret := system.GetEnvOrFail("GOOGLE_CLIENT_SECRET")

	session.NewSession([]byte(clientSecret))

	scopes := gauss.ScopeStrings([]gauss.Scope{gauss.ScopeProfile, gauss.ScopeEmail, gauss.ScopeYouTubeReadonly})
	authService, err := gauss.NewService(googleClientID, googleClientSecret, baseURL, mainPagePath, scopes, *loginTemplateFlag)
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v", err)
	}

	authHandlers, err := gauss.NewHandlers(authService)
	if err != nil {
		log.Fatalf("Failed to initialize handlers: %v", err)
	}

	mux := http.NewServeMux()
	authHandlers.RegisterRoutes(mux)

	templates, err := template.ParseGlob("examples/youtube_listing/templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	mux.Handle(mainPagePath, requestLogger(gauss.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderYouTube(w, r, authService, templates)
	}))))

	mux.Handle(Root, gauss.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, mainPagePath, http.StatusFound)
	})))

	log.Printf("Server starting on port %s", "8080")
	if err := http.ListenAndServe("localhost:8080", mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Request started: method=%s path=%s user_agent=%s remote_addr=%s", r.Method, r.URL.Path, r.UserAgent(), r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("Request completed: path=%s duration=%v", r.URL.Path, time.Since(start))
	})
}

func renderYouTube(w http.ResponseWriter, r *http.Request, svc *gauss.Service, tmpl *template.Template) {
	log.Printf("YouTube render started: user_agent=%s", r.UserAgent())

	sess, err := session.Store().Get(r, constants.SessionName)
	if err != nil {
		log.Printf("Session get failed: %v", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	tokJSON, ok := sess.Values[constants.SessionKeyOAuthToken].(string)
	if !ok {
		log.Printf("OAuth token missing from session: session_id=%s", sess.ID)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokJSON), &token); err != nil {
		log.Printf("Token unmarshal failed: %v", err)
		http.Error(w, "Invalid authentication token", http.StatusInternalServerError)
		return
	}

	log.Printf("Token details: has_access_token=%v has_refresh_token=%v is_expired=%v",
		token.AccessToken != "", token.RefreshToken != "", token.Expiry.Before(time.Now()))

	if token.AccessToken == "" {
		log.Printf("Empty access token")
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	httpClient := svc.GetClient(r.Context(), &token)
	ytService, err := youtube.NewService(r.Context(), option.WithHTTPClient(httpClient))
	if err != nil {
		log.Printf("YouTube service creation failed: %v", err)
		http.Error(w, "YouTube service unavailable", http.StatusInternalServerError)
		return
	}

	channels, err := ytService.Channels.List([]string{"contentDetails"}).Mine(true).Do()
	if err != nil {
		log.Printf("YouTube channels fetch failed: %v is_oauth_error=%v", err, isOAuthError(err))
		if isOAuthError(err) {
			log.Printf("OAuth error detected, redirecting to logout")
			http.Redirect(w, r, "/logout", http.StatusFound)
			return
		}
		http.Error(w, "Failed to access YouTube data", http.StatusInternalServerError)
		return
	}

	if len(channels.Items) == 0 {
		log.Printf("No YouTube channels found")
		http.Error(w, "No YouTube channel found", http.StatusNotFound)
		return
	}

	uploads := channels.Items[0].ContentDetails.RelatedPlaylists.Uploads
	vids, err := ytService.PlaylistItems.List([]string{"snippet"}).PlaylistId(uploads).MaxResults(10).Do()
	if err != nil {
		log.Printf("Playlist items fetch failed: %v", err)
		http.Error(w, "Failed to load videos", http.StatusInternalServerError)
		return
	}

	log.Printf("YouTube videos loaded successfully: count=%d", len(vids.Items))

	if err := tmpl.ExecuteTemplate(w, "youtube_videos.html", vids.Items); err != nil {
		log.Printf("Template execution failed: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
}

func isOAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "invalid_grant") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "access_denied") ||
		strings.Contains(errStr, "invalid_token")
}

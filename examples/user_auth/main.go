package main

import (
	"flag"
	"github.com/temirov/GAuss/examples/user_auth/pkg/dash"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/gauss"
	"github.com/temirov/GAuss/pkg/session"
	"github.com/temirov/utils/system"
)

const (
	DashboardPath        = "/dashboard"
	Root                 = "/"
	defaultPublicBaseURL = "http://localhost:8080"
)

func main() {
	// Define a flag to pass a custom login template path.
	loginTemplateFlag := flag.String("template", "", "Path to custom login template (empty for default)")
	flag.Parse()

	clientSecret := system.GetEnvOrFail("SESSION_SECRET")
	googleClientID := system.GetEnvOrFail("GOOGLE_CLIENT_ID")
	googleClientSecret := system.GetEnvOrFail("GOOGLE_CLIENT_SECRET")

	session.NewSession([]byte(clientSecret))

	customLoginTemplate := *loginTemplateFlag

	publicBaseURL := determinePublicBaseURL()

	authService, err := gauss.NewService(googleClientID, googleClientSecret, publicBaseURL, DashboardPath, gauss.ScopeStrings(gauss.DefaultScopes), customLoginTemplate)
	if err != nil {
		log.Fatalf("Failed to initialize auth service: %v", err)
	}

	authHandlers, err := gauss.NewHandlers(authService)
	if err != nil {
		log.Fatalf("Failed to initialize handlers: %v", err)
	}

	// Set up routing.
	mux := http.NewServeMux()

	// Auth routes (unprotected).
	authHandlers.RegisterRoutes(mux)

	// Initialize dashboard service and handlers.
	pattern := filepath.Join("examples", "user_auth", "templates", "*.html")
	templates, err := template.ParseGlob(pattern)
	if err != nil {
		log.Fatal(err)
	}
	dashService := dash.NewService()
	dashHandlers := dash.NewHandlers(dashService, templates)

	mux.Handle(DashboardPath, gauss.AuthMiddleware(http.HandlerFunc(dashHandlers.Dashboard)))

	// Register root handler with middleware.
	mux.Handle(Root, gauss.AuthMiddleware(http.HandlerFunc(rootHandler)))

	log.Printf("Server starting on :8080 (public base %s)", publicBaseURL)
	log.Fatal(http.ListenAndServe("localhost:8080", mux))
}

func rootHandler(responseWriter http.ResponseWriter, request *http.Request) {
	webSession, _ := session.Store().Get(request, constants.SessionName)
	if webSession.Values[constants.SessionKeyUserEmail] != nil {
		// User is logged in, redirect to dashboard.
		http.Redirect(responseWriter, request, DashboardPath, http.StatusFound)
		return
	}
	// If not logged in, the middleware will handle the redirect to login.
	http.NotFound(responseWriter, request)
}

func determinePublicBaseURL() string {
	envValue := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
	if envValue == "" {
		return defaultPublicBaseURL
	}
	return strings.TrimRight(envValue, "/")
}

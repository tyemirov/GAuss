package gauss

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/sessions"
	"github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"golang.org/x/oauth2"
)

//go:embed templates/*.html
var templatesFileSystem embed.FS

// Handlers bundles the GAuss service, session store, and HTML templates used
// for authentication. Instances of Handlers register HTTP endpoints that
// implement the login and callback workflow.
type Handlers struct {
	service   *Service
	store     *sessions.CookieStore
	templates *template.Template
}

// NewHandlers constructs a Handlers value from a Service. It loads the login
// templates either from the custom path specified on the Service or from the
// embedded templates bundled with GAuss.
func NewHandlers(serviceInstance *Service) (*Handlers, error) {
	var (
		parsedTemplates *template.Template
		err             error
	)
	if serviceInstance.LoginTemplate != "" {
		parsedTemplates, err = template.ParseFiles(serviceInstance.LoginTemplate)
	} else {
		parsedTemplates, err = template.ParseFS(templatesFileSystem, constants.TemplatesPath)
	}
	if err != nil {
		return nil, err
	}

	cookieStore := session.Store()

	return &Handlers{
		service:   serviceInstance,
		store:     cookieStore,
		templates: parsedTemplates,
	}, nil
}

// RegisterRoutes installs the GAuss authentication handlers onto the provided
// ServeMux. It returns the mux for convenience so it can be used inline.
func (handlersInstance *Handlers) RegisterRoutes(httpMux *http.ServeMux) *http.ServeMux {
	httpMux.HandleFunc(constants.LoginPath, handlersInstance.loginHandler)
	httpMux.HandleFunc(constants.GoogleAuthPath, handlersInstance.Login)
	httpMux.HandleFunc(constants.CallbackPath, handlersInstance.Callback)
	httpMux.HandleFunc(constants.LogoutPath, handlersInstance.Logout)

	return httpMux
}

// loginHandler renders the login page. If a custom template was supplied when
// creating the Service it is used; otherwise the embedded template named by
// constants.DefaultTemplateName is executed.
func (handlersInstance *Handlers) loginHandler(responseWriter http.ResponseWriter, request *http.Request) {
	dataMap := map[string]interface{}{
		"error": request.URL.Query().Get("error"),
	}

	var templateName string
	if handlersInstance.service.LoginTemplate != "" {
		templateName = filepath.Base(handlersInstance.service.LoginTemplate)
	} else {
		templateName = constants.DefaultTemplateName
	}

	tmpl := handlersInstance.templates.Lookup(templateName)
	if tmpl == nil {
		http.Error(responseWriter, "Login template not found", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(responseWriter, dataMap); err != nil {
		http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Login initiates the OAuth2 flow with Google by generating a state value,
// storing it in the session and redirecting the user to Google's authorization
// endpoint.
func (handlersInstance *Handlers) Login(responseWriter http.ResponseWriter, request *http.Request) {
	stateValue, stateError := handlersInstance.service.GenerateState()
	if stateError != nil {
		log.Printf("Failed to generate state: %v", stateError)
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	webSession, _ := handlersInstance.store.Get(request, constants.SessionName)
	webSession.Values["oauth_state"] = stateValue
	if sessionSaveError := webSession.Save(request, responseWriter); sessionSaveError != nil {
		log.Printf("Failed to save session: %v", sessionSaveError)
		http.Error(responseWriter, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	oauthConfig := handlersInstance.service.authorizationConfigForRequest(request)

	authorizationURL := oauthConfig.AuthCodeURL(
		stateValue,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	http.Redirect(responseWriter, request, authorizationURL, http.StatusFound)
}

// Callback completes the OAuth2 flow. It validates the state value, exchanges
// the code for a token and stores the retrieved user information in the
// session before redirecting to the configured post-login URL.
func (handlersInstance *Handlers) Callback(responseWriter http.ResponseWriter, request *http.Request) {
	webSession, _ := handlersInstance.store.Get(request, constants.SessionName)
	storedStateValue, stateOk := webSession.Values["oauth_state"].(string)
	if !stateOk {
		log.Println("Missing state in session")
		http.Redirect(responseWriter, request, constants.LoginPath+"?error=missing_state", http.StatusFound)
		return
	}

	receivedStateValue := request.URL.Query().Get("state")
	if storedStateValue != receivedStateValue {
		log.Printf("State mismatch: stored %s vs received %s", storedStateValue, receivedStateValue)
		http.Redirect(responseWriter, request, constants.LoginPath+"?error=invalid_state", http.StatusFound)
		return
	}

	authorizationCode := request.URL.Query().Get("code")
	if authorizationCode == "" {
		log.Println("Missing authorization code")
		http.Redirect(responseWriter, request, constants.LoginPath+"?error=missing_code", http.StatusFound)
		return
	}

	oauthConfig := handlersInstance.service.authorizationConfigForRequest(request)

	oauthToken, tokenExchangeError := oauthConfig.Exchange(request.Context(), authorizationCode)
	if tokenExchangeError != nil {
		log.Printf("Token exchange failed: %v", tokenExchangeError)
		http.Redirect(responseWriter, request, constants.LoginPath+"?error=token_exchange_failed", http.StatusFound)
		return
	}

	if oauthToken.RefreshToken == "" {
		log.Printf("Missing refresh token; re-requesting consent")
		handlersInstance.Login(responseWriter, request)
		return
	}

	hasProfileScope := false
	for _, scope := range oauthConfig.Scopes {
		if scope == string(ScopeProfile) || scope == string(ScopeEmail) {
			hasProfileScope = true
			break
		}
	}

	if hasProfileScope {
		// If profile scopes were requested, fetch user info as before.
		googleUser, getUserError := handlersInstance.service.GetUser(oauthToken)
		if getUserError != nil {
			log.Printf("Failed to get user info: %v", getUserError)
			http.Redirect(responseWriter, request, constants.LoginPath+"?error=user_info_failed", http.StatusFound)
			return
		}
		webSession.Values[constants.SessionKeyUserEmail] = googleUser.Email
		webSession.Values[constants.SessionKeyUserName] = googleUser.Name
		webSession.Values[constants.SessionKeyUserPicture] = googleUser.Picture
	} else {
		// If no profile scopes were requested, the user is still authenticated for API access.
		// We set a generic, non-nil value in the session key that the AuthMiddleware checks.
		// This confirms a valid session exists without needing the user's actual email.
		webSession.Values[constants.SessionKeyUserEmail] = "authenticated_api_user"
	}

	// ALWAYS store the OAuth token, as this is the primary artifact for API-driven apps.
	if tokenBytes, err := json.Marshal(oauthToken); err == nil {
		webSession.Values[constants.SessionKeyOAuthToken] = string(tokenBytes)
	} else {
		log.Printf("Failed to marshal token: %v", err)
	}
	if sessionSaveError := webSession.Save(request, responseWriter); sessionSaveError != nil {
		log.Printf("Failed to save user session: %v", sessionSaveError)
		http.Redirect(responseWriter, request, constants.LoginPath+"?error=session_save_failed", http.StatusFound)
		return
	}

	http.Redirect(responseWriter, request, handlersInstance.service.localRedirectURL, http.StatusFound)
}

// Logout removes all authentication information from the session and redirects
// the client to the login page.
func (handlersInstance *Handlers) Logout(responseWriter http.ResponseWriter, request *http.Request) {
	webSession, _ := handlersInstance.store.Get(request, constants.SessionName)
	webSession.Options.MaxAge = -1
	if webSessionSaveError := webSession.Save(request, responseWriter); webSessionSaveError != nil {
		http.Error(responseWriter, webSessionSaveError.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(responseWriter, request, constants.LoginPath, http.StatusFound)
}

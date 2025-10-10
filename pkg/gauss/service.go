package gauss

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/temirov/GAuss/pkg/constants"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// userInfoEndpoint specifies the URL used to retrieve profile information from
// Google. It is a variable rather than a constant so tests can replace it with
// a mock server endpoint.
var userInfoEndpoint = "https://www.googleapis.com/oauth2/v2/userinfo"

const (
	headerForwarded        = "Forwarded"
	headerXForwardedProto  = "X-Forwarded-Proto"
	headerXForwardedScheme = "X-Forwarded-Scheme"
	headerXForwardedHost   = "X-Forwarded-Host"
	headerXForwardedPort   = "X-Forwarded-Port"
	forwardedProtoPrefix   = "proto="
	forwardedHostPrefix    = "host="
	headerValueSeparator   = ","
	forwardedPairSeparator = ";"
	defaultHTTPScheme      = "https"
)

// GoogleUser represents a user profile retrieved from Google.
type GoogleUser struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// Service encapsulates OAuth2 configuration and redirection settings used by
// GAuss. It generates the authorization URL, validates callbacks and provides
// helper methods for retrieving the authenticated user's profile.
//
// The LoginTemplate field, if non-empty, specifies the HTML template filename
// to be used for the login page instead of the embedded "login.html".
type Service struct {
	config            *oauth2.Config
	publicBaseURL     *url.URL
	callbackPath      *url.URL
	localRedirectURL  string
	logoutRedirectURL string
	LoginTemplate     string
}

// ServiceOption customizes optional behavior when creating a Service.
type ServiceOption func(*Service)

// WithLogoutRedirectURL returns a ServiceOption that overrides the redirect
// destination used after logout. The redirectURL parameter may be either an
// absolute URL or a path relative to the application. Empty values are
// ignored and the default logout redirect is preserved.
func WithLogoutRedirectURL(redirectURL string) ServiceOption {
	return func(serviceInstance *Service) {
		trimmedRedirect := strings.TrimSpace(redirectURL)
		if trimmedRedirect == "" {
			return
		}
		serviceInstance.logoutRedirectURL = trimmedRedirect
	}
}

// NewService initializes a Service with Google OAuth credentials and the local
// redirect URL where authenticated users will be sent after logging in.
// googleOAuthBase should point to the publicly reachable URL of your GAuss
// application (e.g. "http://localhost:8080"). customLoginTemplate may specify
// a login template file to override the default. Additional behavior can be
// customized by passing ServiceOption values.
func NewService(clientID string, clientSecret string, googleOAuthBase string, localRedirectURL string, scopes []string, customLoginTemplate string, options ...ServiceOption) (*Service, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("missing Google OAuth credentials")
	}

	baseURL, googleOAuthBaseErr := url.Parse(googleOAuthBase)
	if googleOAuthBaseErr != nil {
		return nil, errors.New("invalid Google OAuth base URL")
	}
	relativePath, _ := url.Parse(constants.CallbackPath)
	redirectURL := baseURL.ResolveReference(relativePath)

	if len(scopes) == 0 {
		scopes = ScopeStrings(DefaultScopes)
	}

	baseConfig := &oauth2.Config{
		RedirectURL:  redirectURL.String(),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}

	serviceInstance := &Service{
		config:            baseConfig,
		publicBaseURL:     baseURL,
		callbackPath:      relativePath,
		localRedirectURL:  localRedirectURL,
		logoutRedirectURL: constants.LoginPath,
		LoginTemplate:     customLoginTemplate,
	}

	for _, option := range options {
		if option == nil {
			continue
		}
		option(serviceInstance)
	}

	return serviceInstance, nil
}

// GenerateState returns a cryptographically secure random string that is used
// as the OAuth2 state parameter to protect against cross-site request forgery.
func (serviceInstance *Service) GenerateState() (string, error) {
	randomBytes := make([]byte, 32)
	_, readError := rand.Read(randomBytes)
	if readError != nil {
		return "", fmt.Errorf("failed to generate state: %w", readError)
	}
	return base64.URLEncoding.EncodeToString(randomBytes), nil
}

// GetUser contacts Google's userinfo endpoint to retrieve the profile
// associated with the provided OAuth2 token.
func (serviceInstance *Service) GetUser(oauthToken *oauth2.Token) (*GoogleUser, error) {
	httpClient := serviceInstance.config.Client(context.Background(), oauthToken)
	httpResponse, httpError := httpClient.Get(userInfoEndpoint)
	if httpError != nil {
		return nil, fmt.Errorf("failed to get user info: %w", httpError)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API returned status %d", httpResponse.StatusCode)
	}

	var user GoogleUser
	if decodeError := json.NewDecoder(httpResponse.Body).Decode(&user); decodeError != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", decodeError)
	}

	return &user, nil
}

// GetClient creates an authenticated http.Client using the service's OAuth2
// configuration and the provided token.
func (serviceInstance *Service) GetClient(ctx context.Context, token *oauth2.Token) *http.Client {
	return serviceInstance.config.Client(ctx, token)
}

func (serviceInstance *Service) authorizationConfigForRequest(request *http.Request) *oauth2.Config {
	clone := *serviceInstance.config
	clone.RedirectURL = serviceInstance.redirectURLForRequest(request)
	return &clone
}

func (serviceInstance *Service) redirectURLForRequest(request *http.Request) string {
	if serviceInstance.callbackPath == nil {
		return serviceInstance.config.RedirectURL
	}

	baseURL := serviceInstance.effectiveBaseURL(request)
	if baseURL == nil {
		return serviceInstance.config.RedirectURL
	}
	callback := baseURL.ResolveReference(serviceInstance.callbackPath)
	return callback.String()
}

func (serviceInstance *Service) effectiveBaseURL(request *http.Request) *url.URL {
	if serviceInstance.publicBaseURL == nil {
		return nil
	}

	if request == nil {
		return serviceInstance.publicBaseURL
	}

	scheme := serviceInstance.resolveScheme(request)
	host := serviceInstance.resolveHost(request)
	if host == "" {
		return serviceInstance.publicBaseURL
	}

	port := serviceInstance.resolvePort(request)
	if port != "" && !strings.Contains(host, ":") {
		host = host + ":" + port
	}

	baseCopy := *serviceInstance.publicBaseURL
	baseCopy.Scheme = scheme
	baseCopy.Host = host

	return &baseCopy
}

func (serviceInstance *Service) resolveScheme(request *http.Request) string {
	if forwarded := extractForwardedDirective(request.Header.Get(headerForwarded), forwardedProtoPrefix); forwarded != "" {
		return strings.ToLower(forwarded)
	}

	if proto := firstHeaderValue(request.Header.Get(headerXForwardedProto)); proto != "" {
		return strings.ToLower(proto)
	}

	if scheme := firstHeaderValue(request.Header.Get(headerXForwardedScheme)); scheme != "" {
		return strings.ToLower(scheme)
	}

	if request.TLS != nil {
		return defaultHTTPScheme
	}

	if request.URL != nil && request.URL.Scheme != "" {
		return strings.ToLower(request.URL.Scheme)
	}

	if serviceInstance.publicBaseURL != nil && serviceInstance.publicBaseURL.Scheme != "" {
		return strings.ToLower(serviceInstance.publicBaseURL.Scheme)
	}

	return defaultHTTPScheme
}

func (serviceInstance *Service) resolveHost(request *http.Request) string {
	if forwarded := extractForwardedDirective(request.Header.Get(headerForwarded), forwardedHostPrefix); forwarded != "" {
		return forwarded
	}

	if host := firstHeaderValue(request.Header.Get(headerXForwardedHost)); host != "" {
		return host
	}

	if request.Host != "" {
		return request.Host
	}

	if serviceInstance.publicBaseURL != nil {
		return serviceInstance.publicBaseURL.Host
	}

	return ""
}

func (serviceInstance *Service) resolvePort(request *http.Request) string {
	return firstHeaderValue(request.Header.Get(headerXForwardedPort))
}

func firstHeaderValue(headerValue string) string {
	if headerValue == "" {
		return ""
	}

	segments := strings.Split(headerValue, headerValueSeparator)
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractForwardedDirective(headerValue string, prefix string) string {
	if headerValue == "" {
		return ""
	}

	directives := strings.Split(headerValue, headerValueSeparator)
	for _, directive := range directives {
		trimmedDirective := strings.TrimSpace(directive)
		if trimmedDirective == "" {
			continue
		}
		pairs := strings.Split(trimmedDirective, forwardedPairSeparator)
		for _, pair := range pairs {
			trimmedPair := strings.TrimSpace(pair)
			if trimmedPair == "" {
				continue
			}
			lower := strings.ToLower(trimmedPair)
			if !strings.HasPrefix(lower, prefix) {
				continue
			}
			value := strings.TrimSpace(trimmedPair[len(prefix):])
			value = strings.Trim(value, "\"")
			if value != "" {
				return value
			}
		}
	}
	return ""
}

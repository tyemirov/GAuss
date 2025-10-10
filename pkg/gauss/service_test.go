package gauss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/temirov/GAuss/pkg/constants"
	"golang.org/x/oauth2"
)

func TestGenerateStateUnique(t *testing.T) {
	svc, err := NewService("id", "secret", "http://example.com", "/dash", ScopeStrings(DefaultScopes), "")
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}
	a, err := svc.GenerateState()
	if err != nil {
		t.Fatalf("GenerateState error: %v", err)
	}
	b, err := svc.GenerateState()
	if err != nil {
		t.Fatalf("GenerateState error: %v", err)
	}
	if a == b {
		t.Errorf("expected unique states, got %s and %s", a, b)
	}
}

func TestGetUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"email":   "e@example.com",
			"name":    "tester",
			"picture": "img",
		})
	}))
	defer server.Close()

	orig := userInfoEndpoint
	userInfoEndpoint = server.URL
	defer func() { userInfoEndpoint = orig }()

	svc, err := NewService("id", "secret", "http://example.com", "/dash", ScopeStrings(DefaultScopes), "")
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}
	tok := &oauth2.Token{AccessToken: "abc"}
	user, err := svc.GetUser(tok)
	if err != nil {
		t.Fatalf("GetUser error: %v", err)
	}
	if user.Email != "e@example.com" || user.Name != "tester" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestGetClient(t *testing.T) {
	// 1. Create a new service
	svc, err := NewService("id", "secret", "http://example.com", "/dash", nil, "")
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	// 2. Create a dummy token
	tok := &oauth2.Token{
		AccessToken: "test-access-token",
	}

	// 3. Get the client from the service
	client := svc.GetClient(context.Background(), tok)

	// 4. Verify the client is not nil
	if client == nil {
		t.Fatal("GetClient returned a nil client")
	}

	// 5. (Advanced) Verify the transport is an oauth2.Transport
	// This confirms the client is correctly configured for OAuth2.
	if _, ok := client.Transport.(*oauth2.Transport); !ok {
		t.Errorf("Expected client.Transport to be of type *oauth2.Transport, but got %T", client.Transport)
	}
}

func TestNewServiceUsesDefaultLogoutRedirect(t *testing.T) {
	svc, err := NewService("id", "secret", "http://example.com", "/dash", nil, "")
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}
	if svc.logoutRedirectURL != constants.LoginPath {
		t.Fatalf("expected default logout redirect %s, got %s", constants.LoginPath, svc.logoutRedirectURL)
	}
}

func TestNewServiceWithLogoutRedirectOption(t *testing.T) {
	svc, err := NewService("id", "secret", "http://example.com", "/dash", nil, "", WithLogoutRedirectURL("/landing"))
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}
	if svc.logoutRedirectURL != "/landing" {
		t.Fatalf("expected logout redirect /landing, got %s", svc.logoutRedirectURL)
	}
}

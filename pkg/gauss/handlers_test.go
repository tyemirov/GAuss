package gauss

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/temirov/GAuss/pkg/constants"
	"github.com/temirov/GAuss/pkg/session"
	"golang.org/x/oauth2"
)

// helper to create service and handlers for tests
func newTestHandlers(t *testing.T) *Handlers {
	session.NewSession([]byte("secret"))
	svc, err := NewService("id", "secret", "http://localhost:8080", "/dashboard", ScopeStrings(DefaultScopes), "")
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := NewHandlers(svc)
	if err != nil {
		t.Fatal(err)
	}
	return handlers
}

func TestLoginRedirect(t *testing.T) {
	h := newTestHandlers(t)
	req := httptest.NewRequest("GET", constants.GoogleAuthPath, nil)
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Fatal("missing redirect location")
	}
	if len(rr.Header()["Set-Cookie"]) == 0 {
		t.Fatal("expected session cookie")
	}
}

func TestLoginRedirectHonorsForwardedHeaders(t *testing.T) {
	testCases := []struct {
		name       string
		configure  func(*http.Request)
		wantTarget string
	}{
		{
			name: "x-forwarded-proto sets https",
			configure: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			wantTarget: "https://loopaware.mprlab.com/auth/google/callback",
		},
		{
			name: "forwarded header overrides host",
			configure: func(r *http.Request) {
				r.Host = "ignored.example"
				r.Header.Set("Forwarded", "proto=https; host=forwarded.example")
			},
			wantTarget: "https://forwarded.example/auth/google/callback",
		},
		{
			name: "forwarded port appended",
			configure: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
				r.Header.Set("X-Forwarded-Port", "8443")
			},
			wantTarget: "https://loopaware.mprlab.com:8443/auth/google/callback",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			h := newTestHandlers(t)
			req := httptest.NewRequest("GET", constants.GoogleAuthPath, nil)
			req.Host = "loopaware.mprlab.com"
			testCase.configure(req)

			rr := httptest.NewRecorder()
			h.Login(rr, req)

			if rr.Code != http.StatusFound {
				t.Fatalf("expected 302, got %d", rr.Code)
			}

			location := rr.Header().Get("Location")
			if location == "" {
				t.Fatal("missing redirect location")
			}

			locationURL, err := url.Parse(location)
			if err != nil {
				t.Fatalf("failed to parse redirect: %v", err)
			}

			redirectURI := locationURL.Query().Get("redirect_uri")
			if redirectURI != testCase.wantTarget {
				t.Fatalf("expected redirect_uri %s, got %s", testCase.wantTarget, redirectURI)
			}
		})
	}
}

func TestCallbackSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token":"abc","token_type":"bearer","refresh_token":"rtok"}`)
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"email":   "e@example.com",
			"name":    "tester",
			"picture": "pic",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	h := newTestHandlers(t)

	// override endpoints
	h.service.config.Endpoint = oauth2.Endpoint{
		AuthURL:   server.URL + "/auth",
		TokenURL:  server.URL + "/token",
		AuthStyle: oauth2.AuthStyleInParams,
	}

	orig := userInfoEndpoint
	userInfoEndpoint = server.URL + "/userinfo"
	defer func() { userInfoEndpoint = orig }()

	// prepare request with session containing state
	req := httptest.NewRequest("GET", constants.CallbackPath+"?state=s123&code=c1", nil)
	initRR := httptest.NewRecorder()
	sess, _ := session.Store().Get(req, constants.SessionName)
	sess.Values["oauth_state"] = "s123"
	sess.Save(req, initRR)
	cookie := initRR.Result().Cookies()[0]
	req.AddCookie(cookie)

	rr := httptest.NewRecorder()
	h.Callback(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	loc, err := rr.Result().Location()
	if err != nil {
		t.Fatalf("location error: %v", err)
	}
	if loc.Path != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %s", loc.Path)
	}
	// verify session now contains user
	resCookie := rr.Result().Cookies()[0]
	chkReq := httptest.NewRequest("GET", "/", nil)
	chkReq.AddCookie(resCookie)
	sess2, _ := session.Store().Get(chkReq, constants.SessionName)
	if sess2.Values[constants.SessionKeyUserEmail] != "e@example.com" {
		t.Fatalf("user not stored in session")
	}
	if sess2.Values[constants.SessionKeyOAuthToken] == nil {
		t.Fatalf("oauth token not stored")
	}
}

func TestCallbackSuccess_APIOnlyScopes(t *testing.T) {
	// Mock OAuth2 token endpoint. Note: NO /userinfo handler is needed.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"api_token","token_type":"bearer","refresh_token":"rtok"}`)
		} else {
			// If this test calls any other endpoint (like /userinfo), it's a failure.
			t.Fatalf("Unexpected API call to %s", r.URL.Path)
		}
	}))
	defer server.Close()

	// Create service and handlers with a non-profile scope
	session.NewSession([]byte("secret"))
	// Use a dummy API scope for this test
	apiScopes := []string{"https://www.googleapis.com/auth/drive.readonly"}
	svc, err := NewService("id", "secret", "http://localhost:8080", "/dashboard", apiScopes, "")
	if err != nil {
		t.Fatal(err)
	}
	// Override endpoints to point to the mock server
	svc.config.Endpoint = oauth2.Endpoint{
		AuthURL:  server.URL + "/auth",
		TokenURL: server.URL + "/token",
	}
	handlers, err := NewHandlers(svc)
	if err != nil {
		t.Fatal(err)
	}

	// Prepare request with session containing state
	req := httptest.NewRequest("GET", constants.CallbackPath+"?state=s123&code=c1", nil)
	initRR := httptest.NewRecorder()
	sess, _ := session.Store().Get(req, constants.SessionName)
	sess.Values["oauth_state"] = "s123"
	sess.Save(req, initRR)
	cookie := initRR.Result().Cookies()[0]
	req.AddCookie(cookie)

	// Execute the callback
	rr := httptest.NewRecorder()
	handlers.Callback(rr, req)

	// Assertions
	if rr.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
	loc, err := rr.Result().Location()
	if err != nil {
		t.Fatalf("location error: %v", err)
	}
	if loc.Path != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %s", loc.Path)
	}

	// Verify the session now contains the token and the placeholder user,
	// but not the detailed user profile info.
	resCookie := rr.Result().Cookies()[0]
	chkReq := httptest.NewRequest("GET", "/", nil)
	chkReq.AddCookie(resCookie)
	sess2, _ := session.Store().Get(chkReq, constants.SessionName)
	if sess2.Values[constants.SessionKeyOAuthToken] == nil {
		t.Fatalf("oauth token was not stored in session")
	}
	if sess2.Values[constants.SessionKeyUserEmail] != "authenticated_api_user" {
		t.Fatalf("expected placeholder user email, got %v", sess2.Values[constants.SessionKeyUserEmail])
	}
	if sess2.Values[constants.SessionKeyUserName] != nil {
		t.Fatalf("user name should not be stored for API-only scopes")
	}
	if sess2.Values[constants.SessionKeyUserPicture] != nil {
		t.Fatalf("user picture should not be stored for API-only scopes")
	}
}

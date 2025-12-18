package gauss

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
)

func TestAuthMiddlewareRedirects(t *testing.T) {
	session.NewSession([]byte("secret"))
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
}

func TestAuthMiddlewarePasses(t *testing.T) {
	session.NewSession([]byte("secret"))
	req := httptest.NewRequest("GET", "/", nil)
	rrInit := httptest.NewRecorder()
	s, _ := session.Store().Get(req, constants.SessionName)
	s.Values[constants.SessionKeyUserEmail] = "e@example.com"
	s.Save(req, rrInit)
	cookie := rrInit.Result().Cookies()[0]
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d", rr.Code)
	}
}

package gauss

import (
	"github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
	"net/http"
)

// AuthMiddleware ensures that a valid GAuss session exists before allowing the
// request to proceed. Unauthenticated requests are redirected to the login
// page.
func AuthMiddleware(nextHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		webSession, _ := session.Store().Get(request, constants.SessionName)
		if webSession.Values[constants.SessionKeyUserEmail] == nil {
			http.Redirect(responseWriter, request, constants.LoginPath, http.StatusFound)
			return
		}
		nextHandler.ServeHTTP(responseWriter, request)
	})
}

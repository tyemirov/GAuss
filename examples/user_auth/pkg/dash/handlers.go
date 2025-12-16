package dash

import (
	"github.com/tyemirov/GAuss/pkg/constants"
	"github.com/tyemirov/GAuss/pkg/session"
	"html/template"
	"net/http"
)

type Handlers struct {
	service   *Service
	templates *template.Template
}

// NewHandlers returns a handler set for serving the dashboard.
// The service is used to obtain user information from the session and the
// provided templates are executed when rendering the dashboard page.
func NewHandlers(service *Service, templates *template.Template) *Handlers {
	return &Handlers{
		service:   service,
		templates: templates,
	}
}

// Dashboard renders the dashboard.html template using data from the session.
func (handlers *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	webSession, _ := session.Store().Get(r, constants.SessionName)
	data := handlers.service.GetUserData(webSession)
	handlers.templates.ExecuteTemplate(w, "dashboard.html", data)
}

// RegisterRoutes mounts the dashboard route on the supplied ServeMux at the
// provided path.
func (handlers *Handlers) RegisterRoutes(mux *http.ServeMux, path string) {
	mux.HandleFunc(path, handlers.Dashboard)
	// Add other dashboard routes
}

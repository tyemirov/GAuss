package dash

import (
	"github.com/gorilla/sessions"
	"github.com/tyemirov/GAuss/pkg/constants"
)

type Service struct {
	// Add any dashboard-specific dependencies
}

// NewService creates a Service for retrieving user data for the dashboard.
// At the moment it carries no state but is designed to hold future
// dependencies such as database connections.
func NewService() *Service {
	return &Service{}
}

// GetUserData extracts a minimal set of user profile fields from the current
// session and returns them in a map that matches the dashboard template.
func (s *Service) GetUserData(session *sessions.Session) map[string]interface{} {
	return map[string]interface{}{
		"Name":    session.Values[constants.SessionKeyUserName],
		"Email":   session.Values[constants.SessionKeyUserEmail],
		"Picture": session.Values[constants.SessionKeyUserPicture],
	}
}

package dash

import (
	"testing"

	"github.com/gorilla/sessions"
	"github.com/tyemirov/GAuss/pkg/constants"
)

func TestGetUserData(t *testing.T) {
	svc := NewService()
	sess := sessions.NewSession(&sessions.CookieStore{}, constants.SessionName)
	sess.Values = map[interface{}]interface{}{
		constants.SessionKeyUserEmail:   "e@example.com",
		constants.SessionKeyUserName:    "tester",
		constants.SessionKeyUserPicture: "pic",
	}
	data := svc.GetUserData(sess)
	if data["Email"] != "e@example.com" || data["Name"] != "tester" || data["Picture"] != "pic" {
		t.Fatalf("unexpected data: %+v", data)
	}
}

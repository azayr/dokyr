package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJWTSessionRoundTrip(t *testing.T) {
	manager, err := New("a-development-secret-with-more-than-32-characters", "selfhost-test", false)
	if err != nil {
		t.Fatal(err)
	}
	token, err := manager.Token("usr_123", "Test Owner", "owner@example.test", "owner")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	manager.SetCookie(recorder, token)
	request := httptest.NewRequest("GET", "/api/auth/me", nil)
	request.AddCookie(recorder.Result().Cookies()[0])
	claims, err := manager.Parse(request)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "usr_123" || claims.Role != "owner" || claims.Email != "owner@example.test" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestJWTRejectsMissingSession(t *testing.T) {
	manager, _ := New("a-development-secret-with-more-than-32-characters", "selfhost-test", false)
	if _, err := manager.Parse(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Fatal("expected missing token error")
	}
}

func TestTwoFactorChallengeIsNotASession(t *testing.T) {
	manager, _ := New("a-development-secret-with-more-than-32-characters", "selfhost-test", false)
	token, err := manager.ChallengeToken("usr_123")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	manager.SetChallengeCookie(recorder, token)
	request := httptest.NewRequest("POST", "/api/auth/2fa", nil)
	request.AddCookie(recorder.Result().Cookies()[0])
	claims, err := manager.ParseChallenge(request)
	if err != nil || claims.Subject != "usr_123" {
		t.Fatalf("unexpected challenge: %#v, %v", claims, err)
	}
	sessionRequest := httptest.NewRequest("GET", "/api/auth/me", nil)
	sessionRequest.AddCookie(&http.Cookie{Name: CookieName, Value: token})
	if _, err := manager.Parse(sessionRequest); err == nil {
		t.Fatal("two-factor challenge must not authorize a session")
	}
}

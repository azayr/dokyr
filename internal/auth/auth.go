package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	CookieName          = "selfhost_session"
	ChallengeCookieName = "selfhost_2fa_challenge"
)

type contextKey string

const claimsKey contextKey = "claims"

type Claims struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	Purpose string `json:"purpose,omitempty"`
	jwt.RegisteredClaims
}
type Manager struct {
	secret []byte
	issuer string
	secure bool
}

func New(secret, issuer string, secure bool) (*Manager, error) {
	if len(secret) < 32 {
		return nil, errors.New("SELFHOST_JWT_SECRET must be at least 32 characters")
	}
	return &Manager{secret: []byte(secret), issuer: issuer, secure: secure}, nil
}
func (m *Manager) Token(userID, name, email, role string) (string, error) {
	now := time.Now()
	claims := Claims{Name: name, Email: email, Role: role, Purpose: "session", RegisteredClaims: jwt.RegisteredClaims{Subject: userID, Issuer: m.issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour))}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}
func (m *Manager) ChallengeToken(userID string) (string, error) {
	now := time.Now()
	claims := Claims{Purpose: "two_factor", RegisteredClaims: jwt.RegisteredClaims{Subject: userID, Issuer: m.issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute))}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}
func (m *Manager) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: int((12 * time.Hour).Seconds())})
}
func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
func (m *Manager) SetChallengeCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: ChallengeCookieName, Value: token, Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: int((5 * time.Minute).Seconds())})
}
func (m *Manager) ClearChallengeCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: ChallengeCookieName, Value: "", Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}
func (m *Manager) Parse(r *http.Request) (Claims, error) {
	tokenString := ""
	if c, err := r.Cookie(CookieName); err == nil {
		tokenString = c.Value
	}
	if tokenString == "" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		tokenString = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	}
	if tokenString == "" {
		return Claims{}, errors.New("missing token")
	}
	claims, err := m.parseToken(tokenString)
	if err != nil || claims.Purpose == "two_factor" {
		return Claims{}, errors.New("invalid token")
	}
	return claims, nil
}
func (m *Manager) ParseChallenge(r *http.Request) (Claims, error) {
	cookie, err := r.Cookie(ChallengeCookieName)
	if err != nil || cookie.Value == "" {
		return Claims{}, errors.New("missing two-factor challenge")
	}
	claims, err := m.parseToken(cookie.Value)
	if err != nil || claims.Purpose != "two_factor" {
		return Claims{}, errors.New("invalid two-factor challenge")
	}
	return claims, nil
}
func (m *Manager) parseToken(tokenString string) (Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return Claims{}, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}
	return *claims, nil
}
func (m *Manager) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := m.Parse(r)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	})
}
func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(Claims)
	return claims, ok
}
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"authentication required"}`))
}

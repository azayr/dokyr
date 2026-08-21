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

// RoleResolver reports the role a user currently holds. Require consults it on
// every request so that re-roling or deleting an account takes effect
// immediately instead of when the session token expires.
type RoleResolver func(ctx context.Context, userID string) (role string, err error)

type Manager struct {
	secret      []byte
	issuer      string
	secure      bool
	resolveRole RoleResolver
}

func New(secret, issuer string, secure bool) (*Manager, error) {
	if len(secret) < 32 {
		return nil, errors.New("DOKYR_JWT_SECRET must be at least 32 characters")
	}
	return &Manager{secret: []byte(secret), issuer: issuer, secure: secure}, nil
}

// SetRoleResolver installs the authoritative source for a caller's role. Until
// it is set, Require falls back to the role embedded in the session token.
func (m *Manager) SetRoleResolver(resolve RoleResolver) {
	m.resolveRole = resolve
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
		// The token carries a role, but it is only a cached copy that stays
		// valid for the token's lifetime. Re-read the role so that a demoted or
		// deleted account loses access on its next request.
		if m.resolveRole != nil {
			role, err := m.resolveRole(r.Context(), claims.Subject)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			claims.Role = role
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

// WithClaims attaches claims to ctx so FromContext can retrieve them.
func WithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
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

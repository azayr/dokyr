package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/authz"
	"github.com/azayr/selfhost/internal/mailer"
	"github.com/azayr/selfhost/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// invitationLifetime is how long an invitation link stays usable. Longer than a
// password reset because an invitation is often sent ahead of when the person
// gets to it, but still bounded.
const invitationLifetime = 7 * 24 * time.Hour

// userResponse is the shape returned for an account. It deliberately omits the
// password hash and the encrypted 2FA secret, both of which store.User keeps
// unexported from JSON.
type userResponse struct {
	store.User
	Permissions []authz.Permission `json:"permissions"`
}

func (a *API) users(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.Users(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	out := make([]userResponse, 0, len(users))
	for _, u := range users {
		out = append(out, userResponse{User: u, Permissions: authz.Permissions(u.Role)})
	}
	write(w, http.StatusOK, map[string]any{"users": out, "assignableRoles": authz.AssignableRoles()})
}

func (a *API) inviteUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Role = strings.TrimSpace(in.Role)
	if in.Name == "" || len(in.Name) > 120 {
		bad(w, "Enter a name of up to 120 characters.")
		return
	}
	if _, err := mail.ParseAddress(in.Email); err != nil || !strings.Contains(in.Email, "@") || len(in.Email) > 254 {
		bad(w, "Enter a valid email address.")
		return
	}
	if !authz.KnownRole(in.Role) {
		bad(w, "Choose one of the available roles.")
		return
	}

	claims, _ := auth.FromContext(r.Context())

	// The account is created with a random hash nobody holds the plaintext for.
	// Combined with must_set_password it cannot be signed into until the
	// invitation is consumed, so a failure to deliver the email leaves an
	// unusable account rather than one with a guessable password.
	unusable := make([]byte, 32)
	if _, err := rand.Read(unusable); err != nil {
		problem(w, err)
		return
	}
	unusableHash, err := bcrypt.GenerateFromPassword(unusable, bcrypt.DefaultCost)
	if err != nil {
		problem(w, err)
		return
	}
	invited := store.User{
		ID: newID("usr"), Name: in.Name, Email: in.Email, PasswordHash: string(unusableHash),
		Role: in.Role, MustSetPassword: true, InvitedBy: claims.Subject,
	}
	if err := a.store.CreateUser(r.Context(), invited); errors.Is(err, store.ErrEmailTaken) {
		write(w, http.StatusConflict, map[string]string{"error": store.ErrEmailTaken.Error()})
		return
	} else if err != nil {
		problem(w, err)
		return
	}

	inviteURL, err := a.issueInvitationLink(r, invited)
	if err != nil {
		problem(w, err)
		return
	}
	// The link is returned to the owner as well as emailed. SMTP is optional in
	// this deployment, and without the link in the response an owner with no
	// mail server configured would have no way to onboard anyone.
	response := map[string]any{
		"user":          userResponse{User: invited, Permissions: authz.Permissions(invited.Role)},
		"invitationUrl": inviteURL,
		"emailSent":     a.sendInvitationEmail(r, invited, inviteURL),
	}
	a.log.Info("user invited", "user", invited.ID, "role", invited.Role, "by", claims.Subject)
	write(w, http.StatusCreated, response)
}

// resendUserInvitation issues a fresh link for an account that has not yet set a
// password, replacing any earlier one.
func (a *API) resendUserInvitation(w http.ResponseWriter, r *http.Request) {
	target, ok := a.lookupUser(w, r)
	if !ok {
		return
	}
	if !target.MustSetPassword {
		write(w, http.StatusConflict, map[string]string{"error": "This account has already set a password."})
		return
	}
	inviteURL, err := a.issueInvitationLink(r, target)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{
		"invitationUrl": inviteURL,
		"emailSent":     a.sendInvitationEmail(r, target, inviteURL),
	})
}

func (a *API) updateUserRole(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Role = strings.TrimSpace(in.Role)
	if !authz.KnownRole(in.Role) {
		bad(w, "Choose one of the available roles.")
		return
	}
	target, ok := a.lookupUser(w, r)
	if !ok {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	// Changing one's own role is refused rather than merely restricted. An owner
	// demoting themselves would drop the permission mid-request and, if they
	// were the only owner, leave nobody able to restore it.
	if target.ID == claims.Subject {
		write(w, http.StatusConflict, map[string]string{"error": "You cannot change your own role."})
		return
	}
	if err := a.store.UpdateUserRole(r.Context(), target.ID, in.Role); errors.Is(err, store.ErrLastOwner) {
		write(w, http.StatusConflict, map[string]string{"error": store.ErrLastOwner.Error()})
		return
	} else if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	a.log.Info("user role changed", "user", target.ID, "from", target.Role, "to", in.Role, "by", claims.Subject)
	write(w, http.StatusOK, map[string]any{"updated": true, "role": in.Role})
}

func (a *API) deleteUser(w http.ResponseWriter, r *http.Request) {
	target, ok := a.lookupUser(w, r)
	if !ok {
		return
	}
	claims, _ := auth.FromContext(r.Context())
	if target.ID == claims.Subject {
		write(w, http.StatusConflict, map[string]string{"error": "You cannot remove your own account."})
		return
	}
	if err := a.store.DeleteUser(r.Context(), target.ID); errors.Is(err, store.ErrLastOwner) {
		write(w, http.StatusConflict, map[string]string{"error": store.ErrLastOwner.Error()})
		return
	} else if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	// The removed account's session token stays syntactically valid until it
	// expires, but Require resolves the role from the database on every request,
	// so the next one fails authentication.
	a.log.Info("user removed", "user", target.ID, "role", target.Role, "by", claims.Subject)
	write(w, http.StatusOK, map[string]any{"deleted": true})
}

// lookupUser resolves the {id} path value, writing the response on failure.
func (a *API) lookupUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	target, err := a.store.User(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return store.User{}, false
	}
	if err != nil {
		problem(w, err)
		return store.User{}, false
	}
	return target, true
}

// issueInvitationLink mints a one-time link that lets an account set its first
// password. It reuses the password-reset token so there is a single code path
// for verifying and consuming such a link.
func (a *API) issueInvitationLink(r *http.Request, u store.User) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	if err := a.store.CreatePasswordResetToken(r.Context(), hex.EncodeToString(hash[:]), u.ID, time.Now().Add(invitationLifetime)); err != nil {
		return "", err
	}
	return a.publicURL + "/reset-password?token=" + url.QueryEscape(token), nil
}

// sendInvitationEmail delivers the link when SMTP is configured, and reports
// whether it went out. A delivery failure is not an error for the caller: the
// link is also returned in the response.
func (a *API) sendInvitationEmail(r *http.Request, u store.User, inviteURL string) bool {
	settings, config, err := a.smtpMailerConfig(r.Context())
	if err != nil || !settings.Enabled || !smtpConfigured(settings) {
		return false
	}
	name := html.EscapeString(u.Name)
	link := html.EscapeString(inviteURL)
	message := mailer.Message{
		To: u.Email, Subject: "You have been invited to Dokyr",
		Text: "You have been invited to Dokyr as " + u.Role + ".\n\nSet your password using this link (valid for 7 days):\n\n" + inviteURL + "\n",
		HTML: `<div style="font-family:Arial,sans-serif;max-width:620px;margin:auto;padding:32px"><p style="color:#087a51;font-weight:700">DOKYR</p><h1 style="font-size:24px">You have been invited</h1><p>Hello ` + name + `,</p><p>An owner invited you to this Dokyr installation as <strong>` + html.EscapeString(u.Role) + `</strong>. Use the button below to choose a password and sign in. This one-time link expires in 7 days.</p><p style="margin:28px 0"><a href="` + link + `" style="background:#087a51;color:white;text-decoration:none;padding:12px 18px;border-radius:7px;font-weight:700">Set your password</a></p></div>`,
	}
	if err := mailer.Send(r.Context(), config, message); err != nil {
		a.log.Warn("send invitation email", "user", u.ID, "error", err)
		return false
	}
	return true
}

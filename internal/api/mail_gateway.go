package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/mailer"
	"github.com/azayr/selfhost/internal/mailgateway"
	"github.com/azayr/selfhost/internal/store"
)

func (a *API) mailOverview(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	mailSettings, settingsErr := a.store.MailServerSettings(r.Context())
	mailSetup := settingsErr == nil
	if settingsErr != nil && !store.NotFound(settingsErr) {
		problem(w, settingsErr)
		return
	}
	domains, err := a.store.MailDomains(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	keys, err := a.store.MailAPIKeys(r.Context(), claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	messages, err := a.store.MailMessages(r.Context(), claims.Subject, 25)
	if err != nil {
		problem(w, err)
		return
	}
	smtpSettings, _, smtpErr := a.smtpMailerConfig(r.Context())
	stalwartConnected := false
	if mailSetup && a.mailGateway != nil && a.mailGateway.Configured() {
		checkContext, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		stalwartConnected = a.mailGateway.Ping(checkContext) == nil
		cancel()
	}
	write(w, http.StatusOK, map[string]any{
		"domains":            domains,
		"apiKeys":            keys,
		"messages":           messages,
		"stalwartConnected":  stalwartConnected,
		"deliveryConfigured": (stalwartConnected && a.mailGateway.ManagedDelivery()) || (smtpErr == nil && smtpSettings.Enabled && smtpConfigured(smtpSettings)),
		"mailSetup":          mailSetup,
		"mailServerHostname": mailSettings.Hostname,
	})
}

func (a *API) updateMailSetup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Hostname string `json:"hostname"`
	}
	if !decode(w, r, &input) {
		return
	}
	hostname := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(input.Hostname), "."))
	if !mailgateway.ValidPublicHostname(hostname) {
		bad(w, "enter a public mail server hostname such as mail.example.com")
		return
	}
	if a.mailGateway == nil || !a.mailGateway.Configured() {
		write(w, http.StatusServiceUnavailable, map[string]string{"error": "the bundled Stalwart connection is unavailable"})
		return
	}
	a.mailMu.Lock()
	defer a.mailMu.Unlock()
	setupContext, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	restart, err := a.mailGateway.ConfigureServer(setupContext, hostname)
	cancel()
	if err != nil {
		write(w, http.StatusBadGateway, map[string]string{"error": "Stalwart setup failed: " + err.Error()})
		return
	}
	if restart {
		restartContext, restartCancel := context.WithTimeout(r.Context(), 45*time.Second)
		err = a.docker.RestartControlPlaneService(restartContext, "stalwart")
		restartCancel()
		if err != nil {
			write(w, http.StatusBadGateway, map[string]string{"error": "Stalwart was configured but could not restart: " + err.Error()})
			return
		}
	}
	if err := a.waitForMailGateway(r.Context(), 30*time.Second); err != nil {
		write(w, http.StatusBadGateway, map[string]string{"error": "Stalwart was configured but did not become ready: " + err.Error()})
		return
	}
	claims, _ := auth.FromContext(r.Context())
	settings := store.MailServerSettings{Hostname: hostname, CreatedBy: claims.Subject}
	if err := a.store.UpsertMailServerSettings(r.Context(), settings); err != nil {
		problem(w, err)
		return
	}
	refreshed := 0
	domains, err := a.store.AllMailDomains(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	for _, domain := range domains {
		if domain.StalwartID == "" {
			continue
		}
		records, recordErr := a.mailGateway.DomainRecords(r.Context(), domain.StalwartID, domain.Name)
		if recordErr != nil {
			a.log.Warn("refresh mail domain after server setup", "domain", domain.Name, "error", recordErr)
			continue
		}
		if err := a.store.ReplaceMailDNSRecords(r.Context(), domain.ID, records); err != nil {
			a.log.Warn("save refreshed mail DNS records", "domain", domain.Name, "error", err)
			continue
		}
		refreshed++
	}
	saved, err := a.store.MailServerSettings(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"settings": saved, "refreshedDomains": refreshed})
}

func (a *API) waitForMailGateway(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		checkContext, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = a.mailGateway.Ping(checkContext)
		cancel()
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if lastErr == nil {
		lastErr = errors.New("mail service readiness timed out")
	}
	return lastErr
}

func (a *API) createMailDomain(w http.ResponseWriter, r *http.Request) {
	if _, err := a.store.MailServerSettings(r.Context()); store.NotFound(err) {
		write(w, http.StatusConflict, map[string]string{"error": "the platform owner must set up the mail server before domains can be added"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &input) {
		return
	}
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(input.Name), "."))
	if !validMailDomain(name) {
		bad(w, "enter a valid domain name such as example.com")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	token, err := randomMailSecret(24)
	if err != nil {
		problem(w, err)
		return
	}
	domain := store.MailDomain{
		ID: newID("mdom"), UserID: claims.Subject, Name: name,
		Status: "pending_ownership", OwnershipToken: token,
	}
	ownership := store.MailDNSRecord{
		Type: "TXT", Name: "_dokyr-verify." + name, Value: token,
		Purpose: "Ownership", Required: true, Status: "pending",
	}
	if err := a.store.CreateMailDomain(r.Context(), domain, ownership); errors.Is(err, store.ErrMailDomainTaken) {
		write(w, http.StatusConflict, map[string]string{"error": "this domain is already claimed on this Dokyr server"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	domain, err = a.store.MailDomain(r.Context(), domain.ID, claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"domain": domain})
}

func (a *API) verifyMailDomain(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	domain, err := a.store.MailDomain(r.Context(), strings.TrimSpace(r.PathValue("id")), claims.Subject)
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "mail domain not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}

	ownershipVerified := false
	for _, record := range domain.Records {
		checked := a.mailGateway.VerifyRecord(r.Context(), record)
		if err := a.store.UpdateMailDNSRecord(r.Context(), checked); err != nil {
			problem(w, err)
			return
		}
		if checked.Purpose == "Ownership" && checked.Status == "verified" {
			ownershipVerified = true
		}
	}

	if !ownershipVerified {
		_ = a.store.UpdateMailDomainVerification(r.Context(), domain.ID, "pending_ownership", "Add the ownership TXT record before continuing")
		domain, _ = a.store.MailDomain(r.Context(), domain.ID, claims.Subject)
		write(w, http.StatusOK, map[string]any{"domain": domain})
		return
	}

	if domain.StalwartID == "" {
		if a.mailGateway == nil || !a.mailGateway.Configured() {
			_ = a.store.UpdateMailDomainVerification(r.Context(), domain.ID, "pending_dns", "Domain ownership is verified; connect Stalwart to generate the mail records")
			domain, _ = a.store.MailDomain(r.Context(), domain.ID, claims.Subject)
			write(w, http.StatusOK, map[string]any{"domain": domain})
			return
		}
		stalwartID, records, provisionErr := a.mailGateway.ProvisionDomain(r.Context(), domain.Name)
		if provisionErr != nil {
			_ = a.store.UpdateMailDomainVerification(r.Context(), domain.ID, "pending_dns", provisionErr.Error())
			write(w, http.StatusBadGateway, map[string]string{"error": provisionErr.Error()})
			return
		}
		if err := a.store.AddMailDNSRecords(r.Context(), domain.ID, stalwartID, records); err != nil {
			_ = a.mailGateway.DeleteDomain(r.Context(), stalwartID)
			problem(w, err)
			return
		}
		domain, err = a.store.MailDomain(r.Context(), domain.ID, claims.Subject)
		if err != nil {
			problem(w, err)
			return
		}
	}

	allRequiredVerified := true
	for _, record := range domain.Records {
		checked := a.mailGateway.VerifyRecord(r.Context(), record)
		if err := a.store.UpdateMailDNSRecord(r.Context(), checked); err != nil {
			problem(w, err)
			return
		}
		if checked.Required && checked.Status != "verified" {
			allRequiredVerified = false
		}
	}
	status, statusError := "pending_dns", "One or more required DNS records are still missing"
	if allRequiredVerified {
		status, statusError = "verified", ""
	} else if domain.Status == "verified" || domain.Status == "temporary_failure" {
		status, statusError = "temporary_failure", "A required DNS record disappeared after verification"
	}
	if err := a.store.UpdateMailDomainVerification(r.Context(), domain.ID, status, statusError); err != nil {
		problem(w, err)
		return
	}
	domain, err = a.store.MailDomain(r.Context(), domain.ID, claims.Subject)
	if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"domain": domain})
}

func (a *API) deleteMailDomain(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	domain, err := a.store.MailDomain(r.Context(), strings.TrimSpace(r.PathValue("id")), claims.Subject)
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "mail domain not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if domain.StalwartID != "" && a.mailGateway != nil {
		if err := a.mailGateway.DeleteDomain(r.Context(), domain.StalwartID); err != nil {
			write(w, http.StatusBadGateway, map[string]string{"error": "Stalwart could not remove the domain: " + err.Error()})
			return
		}
	}
	if err := a.store.DeleteMailDomain(r.Context(), domain.ID, claims.Subject); err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (a *API) createMailAPIKey(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		DomainID string `json:"domainId"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 80 || strings.ContainsAny(input.Name, "\r\n") {
		bad(w, "enter a key name of at most 80 characters")
		return
	}
	claims, _ := auth.FromContext(r.Context())
	domain, err := a.store.MailDomain(r.Context(), strings.TrimSpace(input.DomainID), claims.Subject)
	if store.NotFound(err) {
		bad(w, "choose a mail domain owned by your account")
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if domain.Status != "verified" {
		write(w, http.StatusConflict, map[string]string{"error": "verify the domain before creating a sending key"})
		return
	}
	secret, err := randomMailSecret(32)
	if err != nil {
		problem(w, err)
		return
	}
	secret = "dkr_mail_" + secret
	sum := sha256.Sum256([]byte(secret))
	key := store.MailAPIKey{ID: newID("mak"), UserID: claims.Subject, DomainID: domain.ID, DomainName: domain.Name,
		Name: input.Name, TokenHash: hex.EncodeToString(sum[:]), TokenPrefix: secret[:16], CreatedAt: time.Now().UTC()}
	if err := a.store.CreateMailAPIKey(r.Context(), key); err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"apiKey": key, "secret": secret})
}

func (a *API) deleteMailAPIKey(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	if err := a.store.DeleteMailAPIKey(r.Context(), strings.TrimSpace(r.PathValue("id")), claims.Subject); store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "mail API key not found"})
		return
	} else if err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *API) sendDeveloperEmail(w http.ResponseWriter, r *http.Request) {
	credential := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if !strings.HasPrefix(credential, "dkr_mail_") {
		write(w, http.StatusUnauthorized, map[string]string{"error": "use a Dokyr mail API key as a bearer token"})
		return
	}
	sum := sha256.Sum256([]byte(credential))
	key, domain, err := a.store.MailDomainByAPIKey(r.Context(), hex.EncodeToString(sum[:]))
	if err != nil {
		write(w, http.StatusUnauthorized, map[string]string{"error": "invalid mail API key"})
		return
	}
	if domain.Status != "verified" {
		write(w, http.StatusForbidden, map[string]string{"error": "the API key's sending domain is not verified"})
		return
	}
	var input struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
		Text    string   `json:"text"`
		ReplyTo string   `json:"replyTo"`
	}
	if !decode(w, r, &input) {
		return
	}
	from, err := mail.ParseAddress(strings.TrimSpace(input.From))
	if err != nil || !strings.EqualFold(addressDomain(from.Address), domain.Name) {
		bad(w, "from must be a valid address on the API key's verified domain")
		return
	}
	if len(input.To) == 0 || len(input.To) > 50 {
		bad(w, "provide between 1 and 50 recipients")
		return
	}
	recipients := make([]string, 0, len(input.To))
	for _, value := range input.To {
		address, parseErr := mail.ParseAddress(strings.TrimSpace(value))
		if parseErr != nil {
			bad(w, "every recipient must be a valid email address")
			return
		}
		recipients = append(recipients, address.Address)
	}
	input.Subject = strings.TrimSpace(input.Subject)
	if input.Subject == "" || len(input.Subject) > 998 || strings.ContainsAny(input.Subject, "\r\n") {
		bad(w, "subject is required and cannot contain line breaks")
		return
	}
	if strings.TrimSpace(input.HTML) == "" && strings.TrimSpace(input.Text) == "" {
		bad(w, "provide an html or text body")
		return
	}
	if strings.TrimSpace(input.ReplyTo) != "" {
		if _, err := mail.ParseAddress(input.ReplyTo); err != nil {
			bad(w, "replyTo must be a valid email address")
			return
		}
	}
	settings, smtpConfig, err := a.smtpMailerConfig(r.Context())
	managedDelivery := a.mailGateway != nil && a.mailGateway.ManagedDelivery() && domain.StalwartID != ""
	if managedDelivery {
		relay, relayErr := a.mailGateway.PrepareSender(r.Context(), domain.StalwartID, from.Address)
		if relayErr != nil {
			write(w, http.StatusBadGateway, map[string]string{"error": "Stalwart could not prepare the sender: " + relayErr.Error()})
			return
		}
		smtpConfig = mailer.Config{
			Host: relay.Host, Port: relay.Port, Encryption: "tls", Username: relay.Username,
			Password: relay.Password, FromName: from.Name, FromEmail: from.Address, InsecureSkipVerify: relay.InsecureSkipVerify,
		}
	} else if err != nil || !settings.Enabled || !smtpConfigured(settings) {
		write(w, http.StatusServiceUnavailable, map[string]string{"error": "mail delivery is not configured in Dokyr"})
		return
	}
	message := store.MailMessage{ID: newID("mail"), DomainID: domain.ID, APIKeyID: key.ID, FromEmail: from.Address,
		FromName: from.Name, Recipients: recipients, Subject: input.Subject, Status: "processing", CreatedAt: time.Now().UTC()}
	if err := a.store.CreateMailMessage(r.Context(), message); err != nil {
		problem(w, err)
		return
	}
	for _, recipient := range recipients {
		delivery := mailer.Message{To: recipient, FromName: from.Name, FromEmail: from.Address, ReplyTo: input.ReplyTo,
			Subject: input.Subject, HTML: input.HTML, Text: input.Text}
		if err := mailer.Send(r.Context(), smtpConfig, delivery); err != nil {
			_ = a.store.CompleteMailMessage(r.Context(), message.ID, "failed", err.Error())
			a.log.Warn("send developer email", "message", message.ID, "domain", domain.Name, "error", err)
			write(w, http.StatusBadGateway, map[string]string{"error": "email delivery failed: " + err.Error(), "id": message.ID})
			return
		}
	}
	if err := a.store.CompleteMailMessage(r.Context(), message.ID, "sent", ""); err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"id": message.ID, "status": "sent"})
}

func randomMailSecret(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validMailDomain(value string) bool {
	if len(value) < 3 || len(value) > 253 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || !strings.Contains(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func addressDomain(address string) string {
	index := strings.LastIndexByte(address, '@')
	if index < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(address[index+1:], "."))
}

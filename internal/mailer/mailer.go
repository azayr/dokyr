package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host       string
	Port       int
	Encryption string
	Username   string
	Password   string
	FromName   string
	FromEmail  string
}

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

func Send(ctx context.Context, config Config, message Message) error {
	config.Host = strings.TrimSpace(config.Host)
	config.FromEmail = strings.TrimSpace(config.FromEmail)
	message.To = strings.TrimSpace(message.To)
	if config.Host == "" || config.Port < 1 || config.Port > 65535 {
		return errors.New("SMTP host and port are required")
	}
	if _, err := mail.ParseAddress(config.FromEmail); err != nil {
		return errors.New("SMTP sender email is invalid")
	}
	if _, err := mail.ParseAddress(message.To); err != nil {
		return errors.New("recipient email is invalid")
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return errors.New("email subject contains invalid characters")
	}

	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	dialer := &net.Dialer{Timeout: 12 * time.Second}
	tlsConfig := &tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12}
	var connection net.Conn
	var err error
	if config.Encryption == "tls" {
		connection, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		connection, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(20 * time.Second))

	client, err := smtp.NewClient(connection, config.Host)
	if err != nil {
		return fmt.Errorf("open SMTP session: %w", err)
	}
	defer client.Close()
	if config.Encryption == "starttls" {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			return errors.New("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	} else if config.Encryption != "tls" && config.Encryption != "none" {
		return errors.New("SMTP encryption must be STARTTLS, TLS, or none")
	}
	if strings.TrimSpace(config.Username) != "" {
		if err := client.Auth(smtp.PlainAuth("", config.Username, config.Password, config.Host)); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(config.FromEmail); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("begin SMTP message: %w", err)
	}
	if _, err := w.Write(buildMessage(config, message)); err != nil {
		w.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func buildMessage(config Config, message Message) []byte {
	boundary := "deployforge-alternative"
	from := (&mail.Address{Name: config.FromName, Address: config.FromEmail}).String()
	to := (&mail.Address{Address: message.To}).String()
	subject := mime.QEncoding.Encode("utf-8", message.Subject)
	var body bytes.Buffer
	fmt.Fprintf(&body, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", from, to, subject, boundary)
	fmt.Fprintf(&body, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", boundary, message.Text)
	fmt.Fprintf(&body, "--%s\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", boundary, message.HTML)
	fmt.Fprintf(&body, "--%s--\r\n", boundary)
	return body.Bytes()
}

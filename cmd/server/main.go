package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/api"
	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/caddy"
	"github.com/azayr/selfhost/internal/config"
	"github.com/azayr/selfhost/internal/integration"
	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/secretbox"
	"github.com/azayr/selfhost/internal/store"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, err := store.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	authManager, err := auth.New(cfg.JWTSecret, cfg.JWTIssuer, cfg.CookieSecure)
	if err != nil {
		log.Error("configure authentication", "error", err)
		os.Exit(1)
	}
	box, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		log.Error("configure credential encryption", "error", err)
		os.Exit(1)
	}
	integrations := integration.New(db, box, integration.Config{
		PublicURL:          cfg.PublicURL,
		GitLabClientID:     cfg.GitLabClientID,
		GitLabClientSecret: cfg.GitLabClientSecret,
		GitLabBaseURL:      cfg.GitLabBaseURL,
	})
	docker, err := runtime.NewDocker()
	if err != nil {
		log.Error("create docker client", "error", err)
		os.Exit(1)
	}
	defer docker.Close()
	metricsContext, stopMetrics := context.WithCancel(context.Background())
	defer stopMetrics()
	if err := docker.StartMetricsCollector(metricsContext); err != nil {
		log.Warn("initial Docker metrics sample failed; collector will retry", "error", err)
	}
	caddyClient, err := caddy.New(cfg.CaddyAdminURL, cfg.ControlHosts)
	if err != nil {
		log.Error("configure Caddy client", "error", err)
		os.Exit(1)
	}
	apiHandler := api.New(db, docker, authManager, integrations, box, caddyClient, cfg.PublicURL, log)
	smtpImported, err := apiHandler.BootstrapSMTPSettings(context.Background(), cfg.SMTP)
	if err != nil {
		log.Error("bootstrap SMTP settings", "error", err)
		os.Exit(1)
	}
	if smtpImported {
		log.Info("SMTP settings imported from environment into PostgreSQL")
	}
	go func() {
		for attempt := 1; attempt <= 10; attempt++ {
			if err := apiHandler.SyncDomains(context.Background()); err == nil {
				log.Info("Caddy domain routes synchronized")
				return
			} else if attempt == 10 {
				log.Warn("could not synchronize Caddy domain routes", "error", err)
			}
			time.Sleep(time.Second)
		}
	}()
	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler.Handler())
	frontend := http.FileServer(http.Dir(cfg.Frontend))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(cfg.Frontend, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			frontend.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(cfg.Frontend, "index.html"))
	})
	log.Info("selfhost listening", "address", cfg.Address)
	if err := http.ListenAndServe(cfg.Address, mux); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

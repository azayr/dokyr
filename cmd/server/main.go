package main

import (
	"context"
	"flag"
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
	"github.com/azayr/selfhost/internal/mailgateway"
	"github.com/azayr/selfhost/internal/platformupdate"
	"github.com/azayr/selfhost/internal/registry"
	"github.com/azayr/selfhost/internal/runtime"
	"github.com/azayr/selfhost/internal/secretbox"
	"github.com/azayr/selfhost/internal/store"
	"github.com/azayr/selfhost/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "update-helper" {
		flags := flag.NewFlagSet("update-helper", flag.ExitOnError)
		container := flags.String("container", "", "running Dokyr container ID")
		targetImage := flags.String("target-image", "", "immutable Dokyr image to install")
		_ = flags.Parse(os.Args[2:])
		if *container == "" || *targetImage == "" {
			slog.Error("update helper requires --container and --target-image")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := runtime.RunPlatformUpdateHelper(ctx, *container, *targetImage); err != nil {
			slog.Error("platform update failed", "error", err)
			os.Exit(1)
		}
		return
	}
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, err := store.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.ReconcilePlatformUpdateJob(context.Background(), version.Current().Version); err != nil && !store.NotFound(err) {
		log.Warn("reconcile interrupted platform update", "error", err)
	}
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
	registryTokens, err := registry.NewTokenIssuer(registry.TokenAuthConfig{
		Issuer:          cfg.RegistryTokenIssuer,
		Service:         cfg.RegistryTokenService,
		PrivateKeyPath:  cfg.RegistryTokenPrivateKeyPath,
		CertificatePath: cfg.RegistryTokenCertificatePath,
	})
	if err != nil {
		log.Error("configure registry token auth", "error", err)
		os.Exit(1)
	}
	docker, err := runtime.NewDocker()
	if err != nil {
		log.Error("create docker client", "error", err)
		os.Exit(1)
	}
	defer docker.Close()
	cleanupContext, stopCleanup := context.WithTimeout(context.Background(), 30*time.Second)
	removedCandidates, cleanupErr := docker.RemoveOrphanApplicationCandidates(cleanupContext)
	stopCleanup()
	if cleanupErr != nil {
		log.Warn("could not remove every orphan application candidate", "error", cleanupErr)
	}
	if len(removedCandidates) > 0 {
		log.Info("removed orphan application candidates", "containers", removedCandidates)
	}
	metricsContext, stopMetrics := context.WithCancel(context.Background())
	defer stopMetrics()
	if err := docker.StartMetricsCollector(metricsContext); err != nil {
		log.Warn("initial Docker metrics sample failed; collector will retry", "error", err)
	}
	caddyClient, err := caddy.New(cfg.CaddyAdminURL, cfg.ControlHosts, cfg.RegistryHosts, cfg.ControlUpstream)
	if err != nil {
		log.Error("configure Caddy client", "error", err)
		os.Exit(1)
	}
	updateClient, err := platformupdate.NewClient(cfg.PlatformImage, cfg.UpdateChannel)
	if err != nil {
		log.Error("configure platform updates", "error", err)
		os.Exit(1)
	}
	mailSetup := false
	if settings, settingsErr := db.MailServerSettings(context.Background()); settingsErr == nil {
		cfg.MailStalwartHostname = settings.Hostname
		cfg.MailStalwartDefaultDomain = settings.Hostname
		mailSetup = true
	} else if !store.NotFound(settingsErr) {
		log.Error("read mail server settings", "error", settingsErr)
		os.Exit(1)
	} else if mailgateway.ValidPublicHostname(cfg.MailStalwartHostname) {
		_, createErr := db.CreateMailServerSettingsIfMissing(context.Background(), store.MailServerSettings{Hostname: cfg.MailStalwartHostname})
		if createErr != nil {
			log.Error("import mail server hostname", "error", createErr)
			os.Exit(1)
		}
		mailSetup = true
		cfg.MailStalwartDefaultDomain = cfg.MailStalwartHostname
	} else {
		cfg.MailStalwartHostname = ""
		cfg.MailStalwartDefaultDomain = ""
	}
	mailGateway, err := mailgateway.New(mailgateway.Config{
		StalwartURL: cfg.MailStalwartURL, StalwartAPIKey: cfg.MailStalwartAPIKey,
		StalwartUser: cfg.MailStalwartUser, StalwartPassword: cfg.MailStalwartPassword,
		BootstrapHostname: cfg.MailStalwartHostname, BootstrapDefaultDomain: cfg.MailStalwartDefaultDomain,
		RelayHost: cfg.MailStalwartRelayHost, RelayPort: cfg.MailStalwartRelayPort, RelayPassword: cfg.MailStalwartRelayPassword,
	})
	if err != nil {
		log.Error("configure mail gateway", "error", err)
		os.Exit(1)
	}
	if mailGateway.Configured() && mailSetup {
		go initializeStalwart(metricsContext, docker, mailGateway, log)
	}
	apiHandler := api.New(db, docker, authManager, integrations, registryTokens, box, caddyClient, updateClient, mailGateway, cfg.PublicURL, cfg.RegistryHosts, cfg.RegistryInternalSecret, log)
	smtpImported, err := apiHandler.BootstrapSMTPSettings(context.Background(), cfg.SMTP)
	if err != nil {
		log.Error("bootstrap SMTP settings", "error", err)
		os.Exit(1)
	}
	if smtpImported {
		log.Info("SMTP settings imported from environment into PostgreSQL")
	}
	registryImported, err := apiHandler.BootstrapRegistrySettings(context.Background(), cfg.Registry)
	if err != nil {
		log.Error("bootstrap registry settings", "error", err)
		os.Exit(1)
	}
	if registryImported {
		log.Info("registry settings imported from environment into PostgreSQL")
	}
	schedulerContext, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	apiHandler.StartCleanupScheduler(schedulerContext)
	apiHandler.StartPlatformUpdateScheduler(schedulerContext)
	apiHandler.StartServerBackupWorker(schedulerContext)
	apiHandler.StartProjectBackupWorker(schedulerContext)
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
	mux.Handle("/v1/", apiHandler.Handler())
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

func initializeStalwart(ctx context.Context, docker *runtime.Docker, gateway *mailgateway.Gateway, log *slog.Logger) {
	for attempt := 1; ; attempt++ {
		attemptContext, cancel := context.WithTimeout(ctx, 20*time.Second)
		restart, err := gateway.EnsureBootstrap(attemptContext)
		cancel()
		if err == nil && restart {
			restartContext, restartCancel := context.WithTimeout(ctx, 45*time.Second)
			err = docker.RestartControlPlaneService(restartContext, "stalwart")
			restartCancel()
		}
		if err == nil {
			readyContext, readyCancel := context.WithTimeout(ctx, 10*time.Second)
			err = gateway.Ping(readyContext)
			readyCancel()
		}
		if err == nil {
			log.Info("bundled Stalwart mail service is ready")
			return
		}
		if attempt == 1 || attempt%12 == 0 {
			log.Warn("waiting for Stalwart mail service", "attempt", attempt, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

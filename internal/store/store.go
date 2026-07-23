package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var ErrAlreadyConfigured = errors.New("selfhost is already configured")

type Store struct{ db *sql.DB }
type User struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	Email                    string    `json:"email"`
	PasswordHash             string    `json:"-"`
	Role                     string    `json:"role"`
	TwoFactorSecretEncrypted string    `json:"-"`
	TwoFactorEnabled         bool      `json:"twoFactorEnabled"`
	GitHubAccountID          string    `json:"-"`
	GitHubLogin              string    `json:"githubLogin,omitempty"`
	CreatedAt                time.Time `json:"createdAt"`
}
type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Repository    string    `json:"repository"`
	Branch        string    `json:"branch"`
	Status        string    `json:"status"`
	Domain        string    `json:"domain"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SourceType    string    `json:"sourceType"`
	ConnectionID  string    `json:"connectionId,omitempty"`
	RegistryID    string    `json:"registryId,omitempty"`
	ImageURL      string    `json:"imageUrl,omitempty"`
	ContainerPort int       `json:"containerPort"`
	HTTPSEnabled  bool      `json:"httpsEnabled"`
}
type ProjectIngressRule struct {
	ID        int64  `json:"id"`
	ProjectID string `json:"-"`
	Path      string `json:"path"`
	Port      int    `json:"port"`
}
type ProjectDomainBindingRule struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	Port      int    `json:"port"`
	ServiceID string `json:"serviceId,omitempty"`
}
type ProjectDomainBinding struct {
	ID           string                     `json:"id"`
	ProjectID    string                     `json:"-"`
	Domain       string                     `json:"domain"`
	HTTPSEnabled bool                       `json:"httpsEnabled"`
	Position     int                        `json:"-"`
	Rules        []ProjectDomainBindingRule `json:"rules"`
}
type SourceConnection struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"-"`
	Provider             string    `json:"provider"`
	AccountID            string    `json:"accountId"`
	AccountName          string    `json:"accountName"`
	AccountAvatar        string    `json:"accountAvatar"`
	BaseURL              string    `json:"baseUrl"`
	AccessTokenEncrypted string    `json:"-"`
	Scopes               string    `json:"scopes"`
	InstallationID       int64     `json:"installationId,omitempty"`
	RepositorySelection  string    `json:"repositorySelection,omitempty"`
	ManageURL            string    `json:"manageUrl,omitempty"`
	ContentsPermission   string    `json:"contentsPermission,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}
type GitHubInstallation struct {
	InstallationID      int64
	ConnectionID        string
	UserID              string
	AccountID           string
	AccountLogin        string
	AccountAvatar       string
	RepositorySelection string
	ManageURL           string
	ContentsPermission  string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
type OAuthState struct {
	StateHash string
	UserID    string
	Provider  string
	ExpiresAt time.Time
}
type AuthOAuthState struct {
	StateHash string
	UserID    string
	Provider  string
	Mode      string
	ExpiresAt time.Time
}
type ProviderSetupState struct {
	StateHash string
	UserID    string
	Provider  string
	Mode      string
	ExpiresAt time.Time
}
type ProviderAppConfig struct {
	Provider               string
	AppID                  string
	AppSlug                string
	ClientIDEncrypted      string
	ClientSecretEncrypted  string
	PrivateKeyEncrypted    string
	WebhookSecretEncrypted string
	CreatedBy              string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
type SMTPSettings struct {
	Enabled                   bool      `json:"enabled"`
	Host                      string    `json:"host"`
	Port                      int       `json:"port"`
	Encryption                string    `json:"encryption"`
	Username                  string    `json:"username"`
	PasswordEncrypted         string    `json:"-"`
	FromName                  string    `json:"fromName"`
	FromEmail                 string    `json:"fromEmail"`
	NotifyDeploymentFailures  bool      `json:"notifyDeploymentFailures"`
	NotifyDeploymentSuccesses bool      `json:"notifyDeploymentSuccesses"`
	CreatedBy                 string    `json:"-"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}
type RegistryCredential struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	RegistryURL       string    `json:"registryUrl"`
	Username          string    `json:"username"`
	PasswordEncrypted string    `json:"-"`
	CreatedBy         string    `json:"-"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
type ApplicationService struct {
	ID                    string    `json:"id"`
	ProjectID             string    `json:"projectId"`
	Name                  string    `json:"name"`
	SourceType            string    `json:"sourceType"`
	ImageURL              string    `json:"imageUrl"`
	RegistryID            string    `json:"registryId,omitempty"`
	ConnectionID          string    `json:"connectionId,omitempty"`
	Repository            string    `json:"repository,omitempty"`
	Branch                string    `json:"branch,omitempty"`
	DockerfilePath        string    `json:"dockerfilePath,omitempty"`
	BuildContext          string    `json:"buildContext,omitempty"`
	BuildStrategy         string    `json:"buildStrategy,omitempty"`
	AutoDeploy            bool      `json:"autoDeploy"`
	RegistryWebhookSecret string    `json:"-"`
	RegistryWebhookTag    string    `json:"registryWebhookTag,omitempty"`
	ContainerPort         int       `json:"containerPort"`
	Command               string    `json:"command,omitempty"`
	HealthCheckType       string    `json:"healthCheckType"`
	HealthCheckPath       string    `json:"healthCheckPath,omitempty"`
	HealthCheckCommand    string    `json:"healthCheckCommand,omitempty"`
	HealthCheckTimeout    int       `json:"healthCheckTimeoutSeconds"`
	Environment           []string  `json:"-"`
	EnvironmentEncrypted  string    `json:"-"`
	EnvironmentSecretKeys []string  `json:"-"`
	Status                string    `json:"status"`
	LastError             string    `json:"lastError,omitempty"`
	Container             string    `json:"container,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}
type Deployment struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	ServiceID   string    `json:"serviceId,omitempty"`
	ServiceName string    `json:"serviceName"`
	Commit      string    `json:"commit"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
	Duration    int       `json:"duration"`
	CreatedAt   time.Time `json:"createdAt"`
}
type DeploymentEvent struct {
	ID           int64     `json:"id"`
	DeploymentID string    `json:"deploymentId"`
	Stage        string    `json:"stage"`
	Type         string    `json:"type"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"createdAt"`
}
type DatabaseService struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"projectId"`
	Name              string    `json:"name"`
	Engine            string    `json:"engine"`
	Image             string    `json:"image"`
	InternalPort      int       `json:"internalPort"`
	PublicEnabled     bool      `json:"publicEnabled"`
	PublicPort        int       `json:"publicPort,omitempty"`
	VolumeName        string    `json:"volumeName"`
	Username          string    `json:"username"`
	DatabaseName      string    `json:"databaseName"`
	PasswordEncrypted string    `json:"-"`
	Status            string    `json:"status"`
	Container         string    `json:"container"`
	InternalAddress   string    `json:"internalAddress"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
type DatabaseDeploymentEvent struct {
	ID                int64     `json:"id"`
	DatabaseServiceID string    `json:"databaseServiceId"`
	Stage             string    `json:"stage"`
	Type              string    `json:"type"`
	Message           string    `json:"message"`
	CreatedAt         time.Time `json:"createdAt"`
}
type ProjectEnvironmentVariable struct {
	ProjectID      string `json:"-"`
	Key            string `json:"key"`
	ValueEncrypted string `json:"-"`
	Secret         bool   `json:"secret"`
	Position       int    `json:"-"`
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	s := &Store{db: db}
	if err := s.wait(ctx, 30*time.Second); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.recoverInterruptedDeployments(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) recoverInterruptedDeployments(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const interruption = "Deployment interrupted by control plane restart"
	if _, err := tx.ExecContext(ctx, `WITH interrupted AS (
			SELECT deployment.id,
				COALESCE((
					SELECT event.stage
					FROM deployment_events event
					WHERE event.deployment_id=deployment.id
					ORDER BY event.id DESC
					LIMIT 1
				), 'prepare') AS stage
			FROM deployments deployment
			WHERE deployment.status IN ('deploying','building')
		)
		INSERT INTO deployment_events(deployment_id,stage,event_type,message)
		SELECT id,stage,'error',$1 FROM interrupted`, interruption); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE application_services
		SET status='failed', last_error=$1, updated_at=NOW()
		WHERE status IN ('deploying','building')`, interruption); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects
		SET status='degraded', updated_at=NOW()
		WHERE status IN ('deploying','building')`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployments
		SET status='failed',
			message=$1,
			duration=GREATEST(duration, FLOOR(EXTRACT(EPOCH FROM (NOW()-created_at)))::INTEGER)
		WHERE status IN ('deploying','building')`, interruption); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) wait(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := s.db.PingContext(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres did not become ready within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "SELECT pg_advisory_lock(7349121)"); err != nil {
		return err
	}
	defer s.db.ExecContext(context.Background(), "SELECT pg_advisory_unlock(7349121)")
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		var applied bool
		if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", entry.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", entry.Name())
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}
func NotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }

func (s *Store) IsConfigured(ctx context.Context) (bool, error) {
	var configured bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users)").Scan(&configured)
	return configured, err
}
func (s *Store) CreateInitialUser(ctx context.Context, u User) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "LOCK TABLE users IN EXCLUSIVE MODE"); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users)").Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrAlreadyConfigured
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO users(id,name,email,password_hash,role) VALUES($1,$2,LOWER($3),$4,'owner')", u.ID, u.Name, u.Email, u.PasswordHash)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id,name,email,password_hash,role,two_factor_secret_encrypted,
		two_factor_enabled,github_account_id,github_login,created_at FROM users WHERE email=LOWER($1)`, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.TwoFactorSecretEncrypted, &u.TwoFactorEnabled, &u.GitHubAccountID, &u.GitHubLogin, &u.CreatedAt)
	return u, err
}
func (s *Store) User(ctx context.Context, id string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id,name,email,password_hash,role,two_factor_secret_encrypted,
		two_factor_enabled,github_account_id,github_login,created_at FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.TwoFactorSecretEncrypted, &u.TwoFactorEnabled, &u.GitHubAccountID, &u.GitHubLogin, &u.CreatedAt)
	return u, err
}

func (s *Store) UserByGitHubAccount(ctx context.Context, accountID string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id,name,email,password_hash,role,two_factor_secret_encrypted,
		two_factor_enabled,github_account_id,github_login,created_at FROM users WHERE github_account_id=$1`, accountID).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.TwoFactorSecretEncrypted, &u.TwoFactorEnabled, &u.GitHubAccountID, &u.GitHubLogin, &u.CreatedAt)
	return u, err
}

func (s *Store) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	result, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash=$2,updated_at=NOW() WHERE id=$1", userID, passwordHash)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) OwnerUser(ctx context.Context) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id,name,email,password_hash,role,two_factor_secret_encrypted,
		two_factor_enabled,github_account_id,github_login,created_at FROM users ORDER BY CASE WHEN role='owner' THEN 0 ELSE 1 END,created_at LIMIT 1`).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.TwoFactorSecretEncrypted, &u.TwoFactorEnabled, &u.GitHubAccountID, &u.GitHubLogin, &u.CreatedAt)
	return u, err
}

func (s *Store) SMTPSettings(ctx context.Context) (SMTPSettings, error) {
	var settings SMTPSettings
	err := s.db.QueryRowContext(ctx, `SELECT enabled,host,port,encryption,username,password_encrypted,from_name,from_email,
		notify_deployment_failures,notify_deployment_successes,COALESCE(created_by,''),created_at,updated_at
		FROM smtp_settings WHERE singleton=TRUE`).Scan(&settings.Enabled, &settings.Host, &settings.Port, &settings.Encryption,
		&settings.Username, &settings.PasswordEncrypted, &settings.FromName, &settings.FromEmail,
		&settings.NotifyDeploymentFailures, &settings.NotifyDeploymentSuccesses, &settings.CreatedBy, &settings.CreatedAt, &settings.UpdatedAt)
	return settings, err
}

func (s *Store) UpsertSMTPSettings(ctx context.Context, settings SMTPSettings) error {
	var createdBy any
	if settings.CreatedBy != "" {
		createdBy = settings.CreatedBy
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO smtp_settings(singleton,enabled,host,port,encryption,username,password_encrypted,from_name,from_email,
		notify_deployment_failures,notify_deployment_successes,created_by)
		VALUES(TRUE,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(singleton) DO UPDATE SET enabled=EXCLUDED.enabled,host=EXCLUDED.host,port=EXCLUDED.port,
		encryption=EXCLUDED.encryption,username=EXCLUDED.username,password_encrypted=EXCLUDED.password_encrypted,
		from_name=EXCLUDED.from_name,from_email=EXCLUDED.from_email,notify_deployment_failures=EXCLUDED.notify_deployment_failures,
		notify_deployment_successes=EXCLUDED.notify_deployment_successes,created_by=COALESCE(smtp_settings.created_by,EXCLUDED.created_by),updated_at=NOW()`,
		settings.Enabled, settings.Host, settings.Port, settings.Encryption, settings.Username, settings.PasswordEncrypted,
		settings.FromName, settings.FromEmail, settings.NotifyDeploymentFailures, settings.NotifyDeploymentSuccesses, createdBy)
	return err
}

// CreateSMTPSettingsIfMissing is used only for first-start environment
// bootstrapping. The conflict guard deliberately prevents container restarts
// from replacing settings that already exist in PostgreSQL.
func (s *Store) CreateSMTPSettingsIfMissing(ctx context.Context, settings SMTPSettings) (bool, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO smtp_settings(singleton,enabled,host,port,encryption,username,password_encrypted,from_name,from_email,
		notify_deployment_failures,notify_deployment_successes)
		VALUES(TRUE,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT(singleton) DO NOTHING`,
		settings.Enabled, settings.Host, settings.Port, settings.Encryption, settings.Username, settings.PasswordEncrypted,
		settings.FromName, settings.FromEmail, settings.NotifyDeploymentFailures, settings.NotifyDeploymentSuccesses)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) CreatePasswordResetToken(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM password_reset_tokens WHERE user_id=$1 OR expires_at<=NOW()", userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO password_reset_tokens(token_hash,user_id,expires_at) VALUES($1,$2,$3)", tokenHash, userID, expiresAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ConsumePasswordResetToken(ctx context.Context, tokenHash, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID string
	if err := tx.QueryRowContext(ctx, `DELETE FROM password_reset_tokens WHERE token_hash=$1 AND expires_at>NOW() RETURNING user_id`, tokenHash).Scan(&userID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "UPDATE users SET password_hash=$2,updated_at=NOW() WHERE id=$1", userID, passwordHash)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count == 0 {
		if err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM password_reset_tokens WHERE user_id=$1", userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetTwoFactor(ctx context.Context, userID, encryptedSecret string, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET two_factor_secret_encrypted=$2,
		two_factor_enabled=$3,updated_at=NOW() WHERE id=$1`, userID, encryptedSecret, enabled)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) LinkGitHubAccount(ctx context.Context, userID, accountID, login string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET github_account_id=$2,github_login=$3,
		updated_at=NOW() WHERE id=$1`, userID, accountID, login)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UnlinkGitHubAccount(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET github_account_id='',github_login='',updated_at=NOW() WHERE id=$1`, userID)
	return err
}

func (s *Store) Projects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,repository,branch,status,domain,updated_at,source_type,COALESCE(connection_id,''),COALESCE(registry_id,''),image_url,container_port,https_enabled FROM projects ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Repository, &p.Branch, &p.Status, &p.Domain, &p.UpdatedAt, &p.SourceType, &p.ConnectionID, &p.RegistryID, &p.ImageURL, &p.ContainerPort, &p.HTTPSEnabled); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
func (s *Store) CreateProject(ctx context.Context, p Project) error {
	var connectionID, registryID any
	if p.ConnectionID != "" {
		connectionID = p.ConnectionID
	}
	if p.RegistryID != "" {
		registryID = p.RegistryID
	}
	status := p.Status
	if status == "" {
		status = "healthy"
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO projects(id,name,repository,branch,status,domain,source_type,connection_id,registry_id,image_url,container_port,https_enabled) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", p.ID, p.Name, p.Repository, p.Branch, status, p.Domain, p.SourceType, connectionID, registryID, p.ImageURL, p.ContainerPort, p.HTTPSEnabled)
	return err
}
func (s *Store) Project(ctx context.Context, id string) (Project, error) {
	var p Project
	err := s.db.QueryRowContext(ctx, "SELECT id,name,repository,branch,status,domain,updated_at,source_type,COALESCE(connection_id,''),COALESCE(registry_id,''),image_url,container_port,https_enabled FROM projects WHERE id=$1", id).Scan(&p.ID, &p.Name, &p.Repository, &p.Branch, &p.Status, &p.Domain, &p.UpdatedAt, &p.SourceType, &p.ConnectionID, &p.RegistryID, &p.ImageURL, &p.ContainerPort, &p.HTTPSEnabled)
	return p, err
}

func (s *Store) ProjectByDomain(ctx context.Context, domain string) (Project, error) {
	var p Project
	err := s.db.QueryRowContext(ctx, "SELECT id,name,repository,branch,status,domain,updated_at,source_type,COALESCE(connection_id,''),COALESCE(registry_id,''),image_url,container_port,https_enabled FROM projects WHERE LOWER(domain)=LOWER($1)", domain).Scan(&p.ID, &p.Name, &p.Repository, &p.Branch, &p.Status, &p.Domain, &p.UpdatedAt, &p.SourceType, &p.ConnectionID, &p.RegistryID, &p.ImageURL, &p.ContainerPort, &p.HTTPSEnabled)
	return p, err
}

func (s *Store) UpdateProjectDomain(ctx context.Context, id, domain string, httpsEnabled bool) error {
	result, err := s.db.ExecContext(ctx, "UPDATE projects SET domain=$2,https_enabled=$3,updated_at=NOW() WHERE id=$1", id, domain, httpsEnabled)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateProjectIngress(ctx context.Context, id, domain string, httpsEnabled bool, containerPort int) error {
	result, err := s.db.ExecContext(ctx, "UPDATE projects SET domain=$2,https_enabled=$3,container_port=$4,updated_at=NOW() WHERE id=$1", id, domain, httpsEnabled, containerPort)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ProjectIngressRules(ctx context.Context, projectID string) ([]ProjectIngressRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,path_pattern,upstream_port FROM project_ingress_rules WHERE project_id=$1 ORDER BY length(path_pattern) DESC,path_pattern`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectIngressRule{}
	for rows.Next() {
		var item ProjectIngressRule
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Path, &item.Port); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ProjectIngressDefaultPath(ctx context.Context, projectID string) (string, error) {
	var path string
	err := s.db.QueryRowContext(ctx, `SELECT path_pattern FROM project_ingress_defaults WHERE project_id=$1`, projectID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "/*", nil
	}
	return path, err
}

func (s *Store) UpdateProjectIngressDefaultPath(ctx context.Context, projectID, path string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO project_ingress_defaults(project_id,path_pattern) VALUES($1,$2)
		ON CONFLICT(project_id) DO UPDATE SET path_pattern=EXCLUDED.path_pattern,updated_at=NOW()`, projectID, path)
	return err
}

func (s *Store) ReplaceProjectIngressRules(ctx context.Context, projectID string, rules []ProjectIngressRule) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_ingress_rules WHERE project_id=$1`, projectID); err != nil {
		return err
	}
	for _, rule := range rules {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_ingress_rules(project_id,path_pattern,upstream_port) VALUES($1,$2,$3)`, projectID, rule.Path, rule.Port); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ProjectDomainBindings(ctx context.Context, projectID string) ([]ProjectDomainBinding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,domain,https_enabled,position
		FROM project_domain_bindings WHERE project_id=$1 ORDER BY position,created_at,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bindings := []ProjectDomainBinding{}
	for rows.Next() {
		var binding ProjectDomainBinding
		if err := rows.Scan(&binding.ID, &binding.ProjectID, &binding.Domain, &binding.HTTPSEnabled, &binding.Position); err != nil {
			return nil, err
		}
		binding.Rules = []ProjectDomainBindingRule{}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range bindings {
		ruleRows, err := s.db.QueryContext(ctx, `SELECT id,path_pattern,upstream_port,COALESCE(service_id,'')
			FROM project_domain_binding_rules WHERE binding_id=$1 ORDER BY position,id`, bindings[index].ID)
		if err != nil {
			return nil, err
		}
		for ruleRows.Next() {
			var rule ProjectDomainBindingRule
			if err := ruleRows.Scan(&rule.ID, &rule.Path, &rule.Port, &rule.ServiceID); err != nil {
				ruleRows.Close()
				return nil, err
			}
			bindings[index].Rules = append(bindings[index].Rules, rule)
		}
		if err := ruleRows.Err(); err != nil {
			ruleRows.Close()
			return nil, err
		}
		ruleRows.Close()
	}
	return bindings, nil
}

func (s *Store) ProjectDomainBindingByDomain(ctx context.Context, domain string) (ProjectDomainBinding, error) {
	var binding ProjectDomainBinding
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,domain,https_enabled,position
		FROM project_domain_bindings WHERE LOWER(domain)=LOWER($1)`, domain).
		Scan(&binding.ID, &binding.ProjectID, &binding.Domain, &binding.HTTPSEnabled, &binding.Position)
	return binding, err
}

func (s *Store) ReplaceProjectDomainBindings(ctx context.Context, projectID string, bindings []ProjectDomainBinding) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_domain_bindings WHERE project_id=$1`, projectID); err != nil {
		return err
	}
	for position, binding := range bindings {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_domain_bindings(id,project_id,domain,https_enabled,position)
			VALUES($1,$2,$3,$4,$5)`, binding.ID, projectID, binding.Domain, binding.HTTPSEnabled, position); err != nil {
			return err
		}
		for rulePosition, rule := range binding.Rules {
			var serviceID any
			if rule.ServiceID != "" {
				serviceID = rule.ServiceID
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO project_domain_binding_rules(binding_id,path_pattern,upstream_port,service_id,position)
				VALUES($1,$2,$3,$4,$5)`, binding.ID, rule.Path, rule.Port, serviceID, rulePosition); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func splitEnvironment(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	return strings.Split(value, "\n")
}

func (s *Store) ApplicationServices(ctx context.Context, projectID string) ([]ApplicationService, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,name,source_type,image_url,COALESCE(registry_id,''),COALESCE(connection_id,''),repository,branch,dockerfile_path,build_context,build_strategy,auto_deploy,registry_webhook_secret_encrypted,registry_webhook_tag,container_port,command,health_check_type,health_check_path,health_check_command,health_check_timeout_seconds,environment,environment_secret_keys,status,last_error,created_at,updated_at
		FROM application_services WHERE project_id=$1 ORDER BY created_at,id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ApplicationService{}
	for rows.Next() {
		var item ApplicationService
		var secretKeys string
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.SourceType, &item.ImageURL, &item.RegistryID, &item.ConnectionID, &item.Repository, &item.Branch, &item.DockerfilePath, &item.BuildContext, &item.BuildStrategy, &item.AutoDeploy, &item.RegistryWebhookSecret, &item.RegistryWebhookTag, &item.ContainerPort, &item.Command, &item.HealthCheckType, &item.HealthCheckPath, &item.HealthCheckCommand, &item.HealthCheckTimeout, &item.EnvironmentEncrypted, &secretKeys, &item.Status, &item.LastError, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.EnvironmentSecretKeys = splitEnvironment(secretKeys)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ApplicationService(ctx context.Context, id string) (ApplicationService, error) {
	var item ApplicationService
	var secretKeys string
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,name,source_type,image_url,COALESCE(registry_id,''),COALESCE(connection_id,''),repository,branch,dockerfile_path,build_context,build_strategy,auto_deploy,registry_webhook_secret_encrypted,registry_webhook_tag,container_port,command,health_check_type,health_check_path,health_check_command,health_check_timeout_seconds,environment,environment_secret_keys,status,last_error,created_at,updated_at
		FROM application_services WHERE id=$1`, id).Scan(&item.ID, &item.ProjectID, &item.Name, &item.SourceType, &item.ImageURL, &item.RegistryID, &item.ConnectionID, &item.Repository, &item.Branch, &item.DockerfilePath, &item.BuildContext, &item.BuildStrategy, &item.AutoDeploy, &item.RegistryWebhookSecret, &item.RegistryWebhookTag, &item.ContainerPort, &item.Command, &item.HealthCheckType, &item.HealthCheckPath, &item.HealthCheckCommand, &item.HealthCheckTimeout, &item.EnvironmentEncrypted, &secretKeys, &item.Status, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	item.EnvironmentSecretKeys = splitEnvironment(secretKeys)
	return item, err
}

func (s *Store) CreateApplicationService(ctx context.Context, service ApplicationService) error {
	var registryID any
	if service.RegistryID != "" {
		registryID = service.RegistryID
	}
	var connectionID any
	if service.ConnectionID != "" {
		connectionID = service.ConnectionID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO application_services(id,project_id,name,source_type,image_url,registry_id,connection_id,repository,branch,dockerfile_path,build_context,build_strategy,auto_deploy,registry_webhook_secret_encrypted,registry_webhook_tag,container_port,command,health_check_type,health_check_path,health_check_command,health_check_timeout_seconds,environment,environment_secret_keys,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`, service.ID, service.ProjectID, service.Name, service.SourceType, service.ImageURL, registryID, connectionID, service.Repository, service.Branch, service.DockerfilePath, service.BuildContext, service.BuildStrategy, service.AutoDeploy, service.RegistryWebhookSecret, service.RegistryWebhookTag, service.ContainerPort, service.Command, service.HealthCheckType, service.HealthCheckPath, service.HealthCheckCommand, service.HealthCheckTimeout, service.EnvironmentEncrypted, strings.Join(service.EnvironmentSecretKeys, "\n"), service.Status)
	return err
}

func (s *Store) UpdateApplicationService(ctx context.Context, service ApplicationService) error {
	var registryID any
	if service.RegistryID != "" {
		registryID = service.RegistryID
	}
	var connectionID any
	if service.ConnectionID != "" {
		connectionID = service.ConnectionID
	}
	result, err := s.db.ExecContext(ctx, `UPDATE application_services
		SET name=$2,source_type=$3,image_url=$4,registry_id=$5,connection_id=$6,repository=$7,branch=$8,dockerfile_path=$9,build_context=$10,build_strategy=$11,container_port=$12,command=$13,health_check_type=$14,health_check_path=$15,health_check_command=$16,health_check_timeout_seconds=$17,updated_at=NOW()
		WHERE id=$1`, service.ID, service.Name, service.SourceType, service.ImageURL, registryID, connectionID, service.Repository, service.Branch, service.DockerfilePath, service.BuildContext, service.BuildStrategy, service.ContainerPort, service.Command, service.HealthCheckType, service.HealthCheckPath, service.HealthCheckCommand, service.HealthCheckTimeout)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateApplicationServiceStatus(ctx context.Context, id, status, lastError string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE application_services SET status=$2,last_error=$3,updated_at=NOW() WHERE id=$1`, id, status, lastError)
	return err
}

func (s *Store) UpdateApplicationServiceEnvironment(ctx context.Context, id, encrypted string, secretKeys []string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE application_services SET environment=$2,environment_secret_keys=$3,updated_at=NOW() WHERE id=$1`, id, encrypted, strings.Join(secretKeys, "\n"))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateApplicationServiceDeploymentTriggers(ctx context.Context, id string, autoDeploy bool, registrySecretEncrypted, registryTag string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE application_services
		SET auto_deploy=$2,registry_webhook_secret_encrypted=$3,registry_webhook_tag=$4,updated_at=NOW()
		WHERE id=$1`, id, autoDeploy, registrySecretEncrypted, registryTag)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) AutoDeployRepositoryServices(ctx context.Context, repository, branch string) ([]ApplicationService, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM application_services
		WHERE source_type='repository' AND auto_deploy=TRUE AND LOWER(repository)=LOWER($1) AND branch=$2
		ORDER BY created_at,id`, repository, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ApplicationService{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		item, err := s.ApplicationService(ctx, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ClaimWebhookDelivery(ctx context.Context, provider, deliveryID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO webhook_deliveries(provider,delivery_id) VALUES($1,$2)
		ON CONFLICT(provider,delivery_id) DO NOTHING`, provider, deliveryID)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count > 0 {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM webhook_deliveries WHERE received_at < NOW() - INTERVAL '30 days'`)
	}
	return count > 0, nil
}

func (s *Store) ApplicationServiceRouteCount(ctx context.Context, id string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_domain_binding_rules WHERE service_id=$1`, id).Scan(&count)
	return count, err
}

func (s *Store) DeleteApplicationService(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM application_services WHERE id=$1`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateProject(ctx context.Context, p Project) error {
	var connectionID, registryID any
	if p.ConnectionID != "" {
		connectionID = p.ConnectionID
	}
	if p.RegistryID != "" {
		registryID = p.RegistryID
	}
	result, err := s.db.ExecContext(ctx, `UPDATE projects
		SET name=$2,repository=$3,branch=$4,source_type=$5,connection_id=$6,registry_id=$7,image_url=$8,container_port=$9,updated_at=NOW()
		WHERE id=$1`, p.ID, p.Name, p.Repository, p.Branch, p.SourceType, connectionID, registryID, p.ImageURL, p.ContainerPort)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id=$1", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ProjectEnvironmentVariables(ctx context.Context, projectID string) ([]ProjectEnvironmentVariable, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id,key,value_encrypted,is_secret,position
		FROM project_environment_variables WHERE project_id=$1 ORDER BY position,key`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	variables := []ProjectEnvironmentVariable{}
	for rows.Next() {
		var variable ProjectEnvironmentVariable
		if err := rows.Scan(&variable.ProjectID, &variable.Key, &variable.ValueEncrypted, &variable.Secret, &variable.Position); err != nil {
			return nil, err
		}
		variables = append(variables, variable)
	}
	return variables, rows.Err()
}

func (s *Store) ReplaceProjectEnvironmentVariables(ctx context.Context, projectID string, variables []ProjectEnvironmentVariable) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM project_environment_variables WHERE project_id=$1", projectID); err != nil {
		return err
	}
	for position, variable := range variables {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_environment_variables(project_id,key,value_encrypted,is_secret,position)
			VALUES($1,$2,$3,$4,$5)`, projectID, variable.Key, variable.ValueEncrypted, variable.Secret, position); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SaveOAuthState(ctx context.Context, state OAuthState) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO oauth_states(state_hash,user_id,provider,expires_at) VALUES($1,$2,$3,$4)", state.StateHash, state.UserID, state.Provider, state.ExpiresAt)
	return err
}

func (s *Store) ConsumeOAuthState(ctx context.Context, stateHash, provider string) (OAuthState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return OAuthState{}, err
	}
	defer tx.Rollback()
	var state OAuthState
	err = tx.QueryRowContext(ctx, "DELETE FROM oauth_states WHERE state_hash=$1 AND provider=$2 AND expires_at>NOW() RETURNING state_hash,user_id,provider,expires_at", stateHash, provider).Scan(&state.StateHash, &state.UserID, &state.Provider, &state.ExpiresAt)
	if err != nil {
		return OAuthState{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM oauth_states WHERE expires_at<=NOW()"); err != nil {
		return OAuthState{}, err
	}
	return state, tx.Commit()
}

func (s *Store) SaveAuthOAuthState(ctx context.Context, state AuthOAuthState) error {
	var userID any
	if state.UserID != "" {
		userID = state.UserID
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO auth_oauth_states(state_hash,user_id,provider,mode,expires_at)
		VALUES($1,$2,$3,$4,$5)`, state.StateHash, userID, state.Provider, state.Mode, state.ExpiresAt)
	return err
}

func (s *Store) ConsumeAuthOAuthState(ctx context.Context, stateHash, provider string) (AuthOAuthState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AuthOAuthState{}, err
	}
	defer tx.Rollback()
	var state AuthOAuthState
	err = tx.QueryRowContext(ctx, `DELETE FROM auth_oauth_states
		WHERE state_hash=$1 AND provider=$2 AND expires_at>NOW()
		RETURNING state_hash,COALESCE(user_id,''),provider,mode,expires_at`, stateHash, provider).
		Scan(&state.StateHash, &state.UserID, &state.Provider, &state.Mode, &state.ExpiresAt)
	if err != nil {
		return AuthOAuthState{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM auth_oauth_states WHERE expires_at<=NOW()"); err != nil {
		return AuthOAuthState{}, err
	}
	return state, tx.Commit()
}

func (s *Store) SaveProviderSetupState(ctx context.Context, state ProviderSetupState) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO provider_setup_states(state_hash,user_id,provider,mode,expires_at)
		VALUES($1,$2,$3,$4,$5)`, state.StateHash, state.UserID, state.Provider, state.Mode, state.ExpiresAt)
	return err
}

func (s *Store) ConsumeProviderSetupState(ctx context.Context, stateHash, provider string) (ProviderSetupState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProviderSetupState{}, err
	}
	defer tx.Rollback()
	var state ProviderSetupState
	err = tx.QueryRowContext(ctx, `DELETE FROM provider_setup_states
		WHERE state_hash=$1 AND provider=$2 AND expires_at>NOW()
		RETURNING state_hash,user_id,provider,mode,expires_at`, stateHash, provider).
		Scan(&state.StateHash, &state.UserID, &state.Provider, &state.Mode, &state.ExpiresAt)
	if err != nil {
		return ProviderSetupState{}, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM provider_setup_states WHERE expires_at<=NOW()"); err != nil {
		return ProviderSetupState{}, err
	}
	return state, tx.Commit()
}

func (s *Store) ProviderAppConfig(ctx context.Context, provider string) (ProviderAppConfig, error) {
	var config ProviderAppConfig
	err := s.db.QueryRowContext(ctx, `SELECT provider,app_id,app_slug,client_id_encrypted,
		client_secret_encrypted,private_key_encrypted,webhook_secret_encrypted,created_by,created_at,updated_at
		FROM provider_app_configs WHERE provider=$1`, provider).
		Scan(&config.Provider, &config.AppID, &config.AppSlug, &config.ClientIDEncrypted,
			&config.ClientSecretEncrypted, &config.PrivateKeyEncrypted, &config.WebhookSecretEncrypted,
			&config.CreatedBy, &config.CreatedAt, &config.UpdatedAt)
	return config, err
}

func (s *Store) UpsertProviderAppConfig(ctx context.Context, config ProviderAppConfig) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO provider_app_configs(provider,app_id,app_slug,client_id_encrypted,
		client_secret_encrypted,private_key_encrypted,webhook_secret_encrypted,created_by)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(provider) DO UPDATE SET app_id=EXCLUDED.app_id,app_slug=EXCLUDED.app_slug,
		client_id_encrypted=EXCLUDED.client_id_encrypted,client_secret_encrypted=EXCLUDED.client_secret_encrypted,
		private_key_encrypted=EXCLUDED.private_key_encrypted,webhook_secret_encrypted=EXCLUDED.webhook_secret_encrypted,
		created_by=EXCLUDED.created_by,updated_at=NOW()`, config.Provider, config.AppID, config.AppSlug,
		config.ClientIDEncrypted, config.ClientSecretEncrypted, config.PrivateKeyEncrypted,
		config.WebhookSecretEncrypted, config.CreatedBy)
	return err
}

// ResetProviderAppConfig removes credentials and source connections that belong
// to a provider app which no longer exists remotely. User identity links are
// intentionally preserved so the same GitHub identity can be authorized again.
func (s *Store) ResetProviderAppConfig(ctx context.Context, provider string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		"DELETE FROM source_connections WHERE provider=$1",
		"DELETE FROM provider_setup_states WHERE provider=$1",
		"DELETE FROM oauth_states WHERE provider=$1",
		"DELETE FROM auth_oauth_states WHERE provider=$1",
		"DELETE FROM provider_app_configs WHERE provider=$1",
	} {
		if _, err := tx.ExecContext(ctx, statement, provider); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpsertSourceConnection(ctx context.Context, c SourceConnection) error {
	_, err := s.UpsertSourceConnectionReturningID(ctx, c)
	return err
}

func (s *Store) UpsertSourceConnectionReturningID(ctx context.Context, c SourceConnection) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `INSERT INTO source_connections(id,user_id,provider,account_id,account_name,account_avatar,base_url,access_token_encrypted,scopes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(provider,account_id,base_url) DO UPDATE SET user_id=EXCLUDED.user_id,account_name=EXCLUDED.account_name,account_avatar=EXCLUDED.account_avatar,access_token_encrypted=EXCLUDED.access_token_encrypted,scopes=EXCLUDED.scopes,updated_at=NOW()
		RETURNING id`, c.ID, c.UserID, c.Provider, c.AccountID, c.AccountName, c.AccountAvatar, c.BaseURL, c.AccessTokenEncrypted, c.Scopes).Scan(&id)
	return id, err
}

func (s *Store) SourceConnections(ctx context.Context, userID string) ([]SourceConnection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.user_id,c.provider,c.account_id,c.account_name,c.account_avatar,c.base_url,c.scopes,c.created_at,c.updated_at,
		COALESCE(i.installation_id,0),COALESCE(i.repository_selection,''),COALESCE(i.manage_url,''),COALESCE(i.contents_permission,'')
		FROM source_connections c LEFT JOIN github_app_installations i ON i.connection_id=c.id
		WHERE c.user_id=$1 ORDER BY c.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SourceConnection{}
	for rows.Next() {
		var c SourceConnection
		if err := rows.Scan(&c.ID, &c.UserID, &c.Provider, &c.AccountID, &c.AccountName, &c.AccountAvatar, &c.BaseURL, &c.Scopes, &c.CreatedAt, &c.UpdatedAt, &c.InstallationID, &c.RepositorySelection, &c.ManageURL, &c.ContentsPermission); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) SourceConnection(ctx context.Context, id, userID string) (SourceConnection, error) {
	var c SourceConnection
	err := s.db.QueryRowContext(ctx, `SELECT c.id,c.user_id,c.provider,c.account_id,c.account_name,c.account_avatar,c.base_url,c.access_token_encrypted,c.scopes,c.created_at,c.updated_at,
		COALESCE(i.installation_id,0),COALESCE(i.repository_selection,''),COALESCE(i.manage_url,''),COALESCE(i.contents_permission,'')
		FROM source_connections c LEFT JOIN github_app_installations i ON i.connection_id=c.id
		WHERE c.id=$1 AND c.user_id=$2`, id, userID).Scan(&c.ID, &c.UserID, &c.Provider, &c.AccountID, &c.AccountName, &c.AccountAvatar, &c.BaseURL, &c.AccessTokenEncrypted, &c.Scopes, &c.CreatedAt, &c.UpdatedAt, &c.InstallationID, &c.RepositorySelection, &c.ManageURL, &c.ContentsPermission)
	return c, err
}

func (s *Store) DeleteSourceConnection(ctx context.Context, id, userID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM source_connections WHERE id=$1 AND user_id=$2", id, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpsertGitHubInstallation(ctx context.Context, installation GitHubInstallation) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO github_app_installations(installation_id,connection_id,user_id,account_id,account_login,account_avatar,repository_selection,manage_url,contents_permission)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(installation_id) DO UPDATE SET connection_id=EXCLUDED.connection_id,user_id=EXCLUDED.user_id,
		account_id=EXCLUDED.account_id,account_login=EXCLUDED.account_login,account_avatar=EXCLUDED.account_avatar,
		repository_selection=EXCLUDED.repository_selection,manage_url=EXCLUDED.manage_url,contents_permission=EXCLUDED.contents_permission,updated_at=NOW()`,
		installation.InstallationID, installation.ConnectionID, installation.UserID, installation.AccountID,
		installation.AccountLogin, installation.AccountAvatar, installation.RepositorySelection, installation.ManageURL, installation.ContentsPermission)
	return err
}

func (s *Store) GitHubInstallation(ctx context.Context, installationID int64) (GitHubInstallation, error) {
	var installation GitHubInstallation
	err := s.db.QueryRowContext(ctx, `SELECT installation_id,connection_id,user_id,account_id,account_login,account_avatar,
		repository_selection,manage_url,contents_permission,created_at,updated_at FROM github_app_installations WHERE installation_id=$1`, installationID).
		Scan(&installation.InstallationID, &installation.ConnectionID, &installation.UserID, &installation.AccountID,
			&installation.AccountLogin, &installation.AccountAvatar, &installation.RepositorySelection,
			&installation.ManageURL, &installation.ContentsPermission, &installation.CreatedAt, &installation.UpdatedAt)
	return installation, err
}

func (s *Store) Registries(ctx context.Context, userID string) ([]RegistryCredential, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id,name,registry_url,username,created_by,created_at,updated_at FROM registry_credentials WHERE created_by=$1 ORDER BY updated_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RegistryCredential{}
	for rows.Next() {
		var c RegistryCredential
		if err := rows.Scan(&c.ID, &c.Name, &c.RegistryURL, &c.Username, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) CreateRegistry(ctx context.Context, c RegistryCredential) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO registry_credentials(id,name,registry_url,username,password_encrypted,created_by) VALUES($1,$2,$3,$4,$5,$6)", c.ID, c.Name, c.RegistryURL, c.Username, c.PasswordEncrypted, c.CreatedBy)
	return err
}

func (s *Store) Registry(ctx context.Context, id, userID string) (RegistryCredential, error) {
	var c RegistryCredential
	err := s.db.QueryRowContext(ctx, "SELECT id,name,registry_url,username,password_encrypted,created_by,created_at,updated_at FROM registry_credentials WHERE id=$1 AND created_by=$2", id, userID).Scan(&c.ID, &c.Name, &c.RegistryURL, &c.Username, &c.PasswordEncrypted, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) DeleteRegistry(ctx context.Context, id, userID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM registry_credentials WHERE id=$1 AND created_by=$2", id, userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) Deployments(ctx context.Context, projectID string) ([]Deployment, error) {
	query := "SELECT id,project_id,COALESCE(service_id,''),service_name,commit_sha,message,status,duration,created_at FROM deployments"
	args := []any{}
	if projectID != "" {
		query += " WHERE project_id=$1"
		args = append(args, projectID)
	}
	query += " ORDER BY created_at DESC LIMIT 20"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Deployment{}
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.ServiceName, &d.Commit, &d.Message, &d.Status, &d.Duration, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
func (s *Store) Deployment(ctx context.Context, id string) (Deployment, error) {
	var d Deployment
	err := s.db.QueryRowContext(ctx, "SELECT id,project_id,COALESCE(service_id,''),service_name,commit_sha,message,status,duration,created_at FROM deployments WHERE id=$1", id).Scan(&d.ID, &d.ProjectID, &d.ServiceID, &d.ServiceName, &d.Commit, &d.Message, &d.Status, &d.Duration, &d.CreatedAt)
	return d, err
}

func (s *Store) CreateDeployment(ctx context.Context, deployment Deployment) error {
	var serviceID any
	if deployment.ServiceID != "" {
		serviceID = deployment.ServiceID
	}
	_, err := s.db.ExecContext(ctx, "INSERT INTO deployments(id,project_id,service_id,service_name,commit_sha,message,status,duration) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", deployment.ID, deployment.ProjectID, serviceID, deployment.ServiceName, deployment.Commit, deployment.Message, deployment.Status, deployment.Duration)
	return err
}

func (s *Store) FinishDeployment(ctx context.Context, id, status, message string, duration int) error {
	_, err := s.db.ExecContext(ctx, "UPDATE deployments SET status=$2,message=$3,duration=$4 WHERE id=$1", id, status, message, duration)
	return err
}

func (s *Store) AppendDeploymentEvent(ctx context.Context, event DeploymentEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO deployment_events(deployment_id,stage,event_type,message)
		VALUES($1,$2,$3,$4)`, event.DeploymentID, event.Stage, event.Type, event.Message)
	return err
}

func (s *Store) DeploymentEvents(ctx context.Context, deploymentID string) ([]DeploymentEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,deployment_id,stage,event_type,message,created_at
		FROM deployment_events WHERE deployment_id=$1 ORDER BY id ASC LIMIT 1000`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DeploymentEvent{}
	for rows.Next() {
		var event DeploymentEvent
		if err := rows.Scan(&event.ID, &event.DeploymentID, &event.Stage, &event.Type, &event.Message, &event.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

func (s *Store) UpdateProjectStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE projects SET status=$2,updated_at=NOW() WHERE id=$1", id, status)
	return err
}

func (s *Store) CreateDatabaseService(ctx context.Context, service DatabaseService) error {
	var publicPort any
	if service.PublicEnabled {
		publicPort = service.PublicPort
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO database_services(id,project_id,name,engine,image,internal_port,public_enabled,public_port,volume_name,username,database_name,password_encrypted)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, service.ID, service.ProjectID, service.Name, service.Engine, service.Image, service.InternalPort, service.PublicEnabled, publicPort, service.VolumeName, service.Username, service.DatabaseName, service.PasswordEncrypted)
	return err
}

func (s *Store) ProjectDatabaseServices(ctx context.Context, projectID string) ([]DatabaseService, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,name,engine,image,internal_port,public_enabled,COALESCE(public_port,0),volume_name,username,database_name,password_encrypted,created_at,updated_at
		FROM database_services WHERE project_id=$1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	services := []DatabaseService{}
	for rows.Next() {
		var service DatabaseService
		if err := rows.Scan(&service.ID, &service.ProjectID, &service.Name, &service.Engine, &service.Image, &service.InternalPort, &service.PublicEnabled, &service.PublicPort, &service.VolumeName, &service.Username, &service.DatabaseName, &service.PasswordEncrypted, &service.CreatedAt, &service.UpdatedAt); err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, rows.Err()
}

func (s *Store) DatabaseService(ctx context.Context, id string) (DatabaseService, error) {
	var service DatabaseService
	err := s.db.QueryRowContext(ctx, `SELECT id,project_id,name,engine,image,internal_port,public_enabled,COALESCE(public_port,0),volume_name,username,database_name,password_encrypted,created_at,updated_at
		FROM database_services WHERE id=$1`, id).Scan(&service.ID, &service.ProjectID, &service.Name, &service.Engine, &service.Image, &service.InternalPort, &service.PublicEnabled, &service.PublicPort, &service.VolumeName, &service.Username, &service.DatabaseName, &service.PasswordEncrypted, &service.CreatedAt, &service.UpdatedAt)
	return service, err
}

func (s *Store) UpdateDatabaseExposure(ctx context.Context, id string, enabled bool, port int) error {
	var publicPort any
	if enabled {
		publicPort = port
	}
	result, err := s.db.ExecContext(ctx, "UPDATE database_services SET public_enabled=$2,public_port=$3,updated_at=NOW() WHERE id=$1", id, enabled, publicPort)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteDatabaseService(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM database_services WHERE id=$1", id)
	return err
}

func (s *Store) AppendDatabaseDeploymentEvent(ctx context.Context, event DatabaseDeploymentEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO database_deployment_events(database_service_id,stage,event_type,message)
		VALUES($1,$2,$3,$4)`, event.DatabaseServiceID, event.Stage, event.Type, event.Message)
	return err
}

func (s *Store) DatabaseDeploymentEvents(ctx context.Context, serviceID string) ([]DatabaseDeploymentEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,database_service_id,stage,event_type,message,created_at
		FROM database_deployment_events WHERE database_service_id=$1 ORDER BY id ASC LIMIT 1000`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DatabaseDeploymentEvent{}
	for rows.Next() {
		var event DatabaseDeploymentEvent
		if err := rows.Scan(&event.ID, &event.DatabaseServiceID, &event.Stage, &event.Type, &event.Message, &event.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

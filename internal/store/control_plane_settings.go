package store

import "context"

type ControlPlaneSettings struct {
	Domain             string `json:"domain"`
	OriginHTTPSEnabled bool   `json:"originHttpsEnabled"`
}

func (s *Store) ControlPlaneSettings(ctx context.Context) (ControlPlaneSettings, error) {
	var settings ControlPlaneSettings
	err := s.db.QueryRowContext(ctx, `SELECT domain,origin_https_enabled FROM control_plane_settings WHERE singleton=TRUE`).Scan(&settings.Domain, &settings.OriginHTTPSEnabled)
	return settings, err
}

func (s *Store) SaveControlPlaneSettings(ctx context.Context, settings ControlPlaneSettings) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO control_plane_settings(singleton,domain,origin_https_enabled,updated_at)
		VALUES(TRUE,$1,$2,NOW()) ON CONFLICT(singleton) DO UPDATE SET domain=EXCLUDED.domain,origin_https_enabled=EXCLUDED.origin_https_enabled,updated_at=NOW()`, settings.Domain, settings.OriginHTTPSEnabled)
	return err
}

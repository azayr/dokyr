package store

import "context"

type ControlPlaneSettings struct {
	Domain string `json:"domain"`
}

func (s *Store) ControlPlaneSettings(ctx context.Context) (ControlPlaneSettings, error) {
	var settings ControlPlaneSettings
	err := s.db.QueryRowContext(ctx, `SELECT domain FROM control_plane_settings WHERE singleton=TRUE`).Scan(&settings.Domain)
	return settings, err
}

func (s *Store) SaveControlPlaneSettings(ctx context.Context, settings ControlPlaneSettings) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO control_plane_settings(singleton,domain,updated_at)
		VALUES(TRUE,$1,NOW()) ON CONFLICT(singleton) DO UPDATE SET domain=EXCLUDED.domain,updated_at=NOW()`, settings.Domain)
	return err
}

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type MailDomain struct {
	ID             string          `json:"id"`
	UserID         string          `json:"-"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	OwnershipToken string          `json:"-"`
	StalwartID     string          `json:"-"`
	LastError      string          `json:"lastError,omitempty"`
	LastCheckedAt  *time.Time      `json:"lastCheckedAt"`
	VerifiedAt     *time.Time      `json:"verifiedAt"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Records        []MailDNSRecord `json:"records"`
}

type MailDNSRecord struct {
	ID        int64      `json:"id"`
	DomainID  string     `json:"-"`
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Value     string     `json:"value"`
	Priority  int        `json:"priority,omitempty"`
	Purpose   string     `json:"purpose"`
	Required  bool       `json:"required"`
	Status    string     `json:"status"`
	LastError string     `json:"lastError,omitempty"`
	CheckedAt *time.Time `json:"checkedAt"`
}

type MailAPIKey struct {
	ID          string     `json:"id"`
	UserID      string     `json:"-"`
	DomainID    string     `json:"domainId"`
	DomainName  string     `json:"domainName,omitempty"`
	Name        string     `json:"name"`
	TokenHash   string     `json:"-"`
	TokenPrefix string     `json:"prefix"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type MailMessage struct {
	ID         string     `json:"id"`
	DomainID   string     `json:"domainId"`
	APIKeyID   string     `json:"-"`
	FromEmail  string     `json:"from"`
	FromName   string     `json:"fromName,omitempty"`
	Recipients []string   `json:"to"`
	Subject    string     `json:"subject"`
	Status     string     `json:"status"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	SentAt     *time.Time `json:"sentAt"`
}

func (s *Store) CreateMailDomain(ctx context.Context, domain MailDomain, ownership MailDNSRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO mail_domains(id,user_id,name,status,ownership_token)
		VALUES($1,$2,LOWER($3),$4,$5)`, domain.ID, domain.UserID, domain.Name, domain.Status, domain.OwnershipToken)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrMailDomainTaken
		}
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO mail_domain_dns_records(domain_id,record_type,name,value,priority,purpose,required)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, domain.ID, ownership.Type, ownership.Name, ownership.Value,
		ownership.Priority, ownership.Purpose, ownership.Required)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MailDomains(ctx context.Context, userID string) ([]MailDomain, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,name,status,ownership_token,stalwart_id,last_error,
		last_checked_at,verified_at,created_at,updated_at FROM mail_domains WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	domains := []MailDomain{}
	for rows.Next() {
		var domain MailDomain
		if err := rows.Scan(&domain.ID, &domain.UserID, &domain.Name, &domain.Status, &domain.OwnershipToken,
			&domain.StalwartID, &domain.LastError, &domain.LastCheckedAt, &domain.VerifiedAt, &domain.CreatedAt, &domain.UpdatedAt); err != nil {
			return nil, err
		}
		domain.Records, err = s.MailDNSRecords(ctx, domain.ID)
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

func (s *Store) MailDomain(ctx context.Context, id, userID string) (MailDomain, error) {
	var domain MailDomain
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,name,status,ownership_token,stalwart_id,last_error,
		last_checked_at,verified_at,created_at,updated_at FROM mail_domains WHERE id=$1 AND user_id=$2`, id, userID).
		Scan(&domain.ID, &domain.UserID, &domain.Name, &domain.Status, &domain.OwnershipToken, &domain.StalwartID,
			&domain.LastError, &domain.LastCheckedAt, &domain.VerifiedAt, &domain.CreatedAt, &domain.UpdatedAt)
	if err != nil {
		return domain, err
	}
	domain.Records, err = s.MailDNSRecords(ctx, id)
	return domain, err
}

func (s *Store) MailDNSRecords(ctx context.Context, domainID string) ([]MailDNSRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,domain_id,record_type,name,value,priority,purpose,required,status,last_error,checked_at
		FROM mail_domain_dns_records WHERE domain_id=$1 ORDER BY CASE purpose WHEN 'Ownership' THEN 0 WHEN 'DKIM' THEN 1 WHEN 'SPF' THEN 2 WHEN 'Return path' THEN 3 ELSE 4 END,id`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []MailDNSRecord{}
	for rows.Next() {
		var record MailDNSRecord
		if err := rows.Scan(&record.ID, &record.DomainID, &record.Type, &record.Name, &record.Value, &record.Priority,
			&record.Purpose, &record.Required, &record.Status, &record.LastError, &record.CheckedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) AddMailDNSRecords(ctx context.Context, domainID, stalwartID string, records []MailDNSRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE mail_domains SET stalwart_id=$2,status='pending_dns',last_error='',updated_at=NOW() WHERE id=$1`, domainID, stalwartID); err != nil {
		return err
	}
	for _, record := range records {
		_, err := tx.ExecContext(ctx, `INSERT INTO mail_domain_dns_records(domain_id,record_type,name,value,priority,purpose,required)
			VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(domain_id,record_type,name,value) DO NOTHING`, domainID,
			record.Type, record.Name, record.Value, record.Priority, record.Purpose, record.Required)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateMailDNSRecord(ctx context.Context, record MailDNSRecord) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mail_domain_dns_records SET status=$2,last_error=$3,checked_at=NOW() WHERE id=$1`, record.ID, record.Status, record.LastError)
	return err
}

func (s *Store) UpdateMailDomainVerification(ctx context.Context, id, status, lastError string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mail_domains SET status=$2,last_error=$3,last_checked_at=NOW(),
		verified_at=CASE WHEN $2='verified' THEN COALESCE(verified_at,NOW()) ELSE verified_at END,updated_at=NOW() WHERE id=$1`, id, status, lastError)
	return err
}

func (s *Store) DeleteMailDomain(ctx context.Context, id, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM mail_domains WHERE id=$1 AND user_id=$2`, id, userID)
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

func (s *Store) MailAPIKeys(ctx context.Context, userID string) ([]MailAPIKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key.id,key.user_id,key.domain_id,domain.name,key.name,key.token_prefix,key.last_used_at,key.created_at
		FROM mail_api_keys key JOIN mail_domains domain ON domain.id=key.domain_id WHERE key.user_id=$1 ORDER BY key.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []MailAPIKey{}
	for rows.Next() {
		var key MailAPIKey
		if err := rows.Scan(&key.ID, &key.UserID, &key.DomainID, &key.DomainName, &key.Name, &key.TokenPrefix, &key.LastUsedAt, &key.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) CreateMailAPIKey(ctx context.Context, key MailAPIKey) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mail_api_keys(id,user_id,domain_id,name,token_hash,token_prefix) VALUES($1,$2,$3,$4,$5,$6)`,
		key.ID, key.UserID, key.DomainID, key.Name, key.TokenHash, key.TokenPrefix)
	return err
}

func (s *Store) DeleteMailAPIKey(ctx context.Context, id, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM mail_api_keys WHERE id=$1 AND user_id=$2`, id, userID)
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

func (s *Store) MailDomainByAPIKey(ctx context.Context, tokenHash string) (MailAPIKey, MailDomain, error) {
	var key MailAPIKey
	var domain MailDomain
	err := s.db.QueryRowContext(ctx, `UPDATE mail_api_keys AS key SET last_used_at=NOW()
		FROM mail_domains AS domain WHERE key.token_hash=$1 AND key.domain_id=domain.id
		RETURNING key.id,key.user_id,key.domain_id,key.name,key.token_hash,key.token_prefix,key.last_used_at,key.created_at,
			domain.id,domain.user_id,domain.name,domain.status,domain.ownership_token,domain.stalwart_id,domain.last_error,
			domain.last_checked_at,domain.verified_at,domain.created_at,domain.updated_at`, tokenHash).
		Scan(&key.ID, &key.UserID, &key.DomainID, &key.Name, &key.TokenHash, &key.TokenPrefix, &key.LastUsedAt, &key.CreatedAt,
			&domain.ID, &domain.UserID, &domain.Name, &domain.Status, &domain.OwnershipToken, &domain.StalwartID,
			&domain.LastError, &domain.LastCheckedAt, &domain.VerifiedAt, &domain.CreatedAt, &domain.UpdatedAt)
	return key, domain, err
}

func (s *Store) CreateMailMessage(ctx context.Context, message MailMessage) error {
	recipients, err := json.Marshal(message.Recipients)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO mail_messages(id,domain_id,api_key_id,from_email,from_name,recipients,subject,status)
		VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8)`, message.ID, message.DomainID, message.APIKeyID, message.FromEmail,
		message.FromName, recipients, message.Subject, message.Status)
	return err
}

func (s *Store) CompleteMailMessage(ctx context.Context, id, status, messageError string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE mail_messages SET status=$2,error=$3,sent_at=CASE WHEN $2='sent' THEN NOW() ELSE NULL END WHERE id=$1`, id, status, messageError)
	return err
}

func (s *Store) MailMessages(ctx context.Context, userID string, limit int) ([]MailMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT message.id,message.domain_id,COALESCE(message.api_key_id,''),message.from_email,message.from_name,
		message.recipients,message.subject,message.status,message.error,message.created_at,message.sent_at
		FROM mail_messages message JOIN mail_domains domain ON domain.id=message.domain_id
		WHERE domain.user_id=$1 ORDER BY message.created_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []MailMessage{}
	for rows.Next() {
		var message MailMessage
		var recipients []byte
		if err := rows.Scan(&message.ID, &message.DomainID, &message.APIKeyID, &message.FromEmail, &message.FromName,
			&recipients, &message.Subject, &message.Status, &message.Error, &message.CreatedAt, &message.SentAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(recipients, &message.Recipients); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

ALTER TABLE mail_api_keys
    ADD COLUMN smtp_username TEXT NOT NULL DEFAULT '',
    ADD COLUMN stalwart_account_id TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX mail_api_keys_smtp_username_unique_idx
    ON mail_api_keys (LOWER(smtp_username)) WHERE smtp_username <> '';

ALTER TABLE mail_server_settings
    ADD COLUMN domain_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN acme_provider_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN stalwart_domain_id TEXT NOT NULL DEFAULT '';

UPDATE mail_server_settings
SET domain_name=CASE WHEN hostname LIKE 'mail.%' THEN SUBSTRING(hostname FROM 6) ELSE hostname END
WHERE domain_name='';

ALTER TABLE mail_messages DROP CONSTRAINT IF EXISTS mail_messages_status_check;
UPDATE mail_messages SET status='queued' WHERE status='sent';
ALTER TABLE mail_messages ADD CONSTRAINT mail_messages_status_check
    CHECK (status IN ('processing','queued','delivered','failed'));

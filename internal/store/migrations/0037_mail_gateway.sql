CREATE TABLE mail_domains (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending_ownership'
        CHECK (status IN ('pending_ownership','pending_dns','verified','temporary_failure','failed')),
    ownership_token TEXT NOT NULL,
    stalwart_id TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX mail_domains_name_unique_idx ON mail_domains (LOWER(name));
CREATE INDEX mail_domains_user_created_idx ON mail_domains (user_id, created_at DESC);

CREATE TABLE mail_domain_dns_records (
    id BIGSERIAL PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES mail_domains(id) ON DELETE CASCADE,
    record_type TEXT NOT NULL CHECK (record_type IN ('TXT','MX','CNAME')),
    name TEXT NOT NULL,
    value TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    purpose TEXT NOT NULL,
    required BOOLEAN NOT NULL DEFAULT TRUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified','missing','incorrect')),
    last_error TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(domain_id, record_type, name, value)
);

CREATE INDEX mail_domain_dns_records_domain_idx ON mail_domain_dns_records(domain_id, id);

CREATE TABLE mail_api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain_id TEXT NOT NULL REFERENCES mail_domains(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX mail_api_keys_user_created_idx ON mail_api_keys(user_id, created_at DESC);

CREATE TABLE mail_messages (
    id TEXT PRIMARY KEY,
    domain_id TEXT NOT NULL REFERENCES mail_domains(id) ON DELETE CASCADE,
    api_key_id TEXT REFERENCES mail_api_keys(id) ON DELETE SET NULL,
    from_email TEXT NOT NULL,
    from_name TEXT NOT NULL DEFAULT '',
    recipients JSONB NOT NULL DEFAULT '[]'::jsonb,
    subject TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing','sent','failed')),
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX mail_messages_domain_created_idx ON mail_messages(domain_id, created_at DESC);

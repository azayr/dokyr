-- Normalize database credentials so a cluster user can be granted access to
-- multiple logical databases and project attachments can select that identity.
CREATE TABLE database_cluster_users (
    id TEXT PRIMARY KEY,
    database_service_id TEXT NOT NULL REFERENCES database_services(id) ON DELETE CASCADE,
    username TEXT NOT NULL,
    password_encrypted TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX database_cluster_users_name_unique
    ON database_cluster_users (database_service_id, LOWER(username));

INSERT INTO database_cluster_users(id,database_service_id,username,password_encrypted,created_at,updated_at)
SELECT DISTINCT ON (database_service_id, LOWER(username))
       id || '_user',database_service_id,username,password_encrypted,created_at,updated_at
FROM database_cluster_databases
ORDER BY database_service_id, LOWER(username), created_at, id;

ALTER TABLE database_cluster_databases ADD COLUMN owner_user_id TEXT;

UPDATE database_cluster_databases database
SET owner_user_id = cluster_user.id
FROM database_cluster_users cluster_user
WHERE cluster_user.database_service_id=database.database_service_id
  AND LOWER(cluster_user.username)=LOWER(database.username);

ALTER TABLE database_cluster_databases
    ALTER COLUMN owner_user_id SET NOT NULL,
    ADD CONSTRAINT database_cluster_databases_owner_user_fkey
        FOREIGN KEY(owner_user_id) REFERENCES database_cluster_users(id) ON DELETE RESTRICT;

CREATE TABLE database_cluster_user_grants (
    database_id TEXT NOT NULL REFERENCES database_cluster_databases(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES database_cluster_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(database_id,user_id)
);

INSERT INTO database_cluster_user_grants(database_id,user_id)
SELECT id,owner_user_id FROM database_cluster_databases;

ALTER TABLE project_database_attachments ADD COLUMN database_user_id TEXT;

UPDATE project_database_attachments attachment
SET database_user_id=database.owner_user_id
FROM database_cluster_databases database
WHERE database.id=attachment.database_id;

ALTER TABLE project_database_attachments
    ALTER COLUMN database_user_id SET NOT NULL,
    ADD CONSTRAINT project_database_attachments_user_fkey
        FOREIGN KEY(database_user_id) REFERENCES database_cluster_users(id) ON DELETE RESTRICT;

CREATE INDEX project_database_attachments_database_idx
    ON project_database_attachments(database_id, database_user_id);

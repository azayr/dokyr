-- Repair clusters created during the transition from project-owned database
-- services to the normalized cluster catalog. The runtime database and login
-- already exist; this restores only missing control-plane metadata.
INSERT INTO database_cluster_users(id,database_service_id,username,password_encrypted,created_at,updated_at)
SELECT cluster.id || '_user',cluster.id,cluster.username,cluster.password_encrypted,cluster.created_at,cluster.updated_at
FROM database_services cluster
WHERE NOT EXISTS (
    SELECT 1 FROM database_cluster_users cluster_user
    WHERE cluster_user.database_service_id=cluster.id
      AND LOWER(cluster_user.username)=LOWER(cluster.username)
)
ON CONFLICT DO NOTHING;

INSERT INTO database_cluster_databases(id,database_service_id,name,owner_user_id,username,password_encrypted,created_at,updated_at)
SELECT cluster.id || '_database',cluster.id,cluster.database_name,cluster_user.id,cluster.username,cluster.password_encrypted,cluster.created_at,cluster.updated_at
FROM database_services cluster
JOIN database_cluster_users cluster_user
  ON cluster_user.database_service_id=cluster.id
 AND LOWER(cluster_user.username)=LOWER(cluster.username)
WHERE NOT EXISTS (
    SELECT 1 FROM database_cluster_databases database
    WHERE database.database_service_id=cluster.id
      AND LOWER(database.name)=LOWER(cluster.database_name)
)
ON CONFLICT DO NOTHING;

INSERT INTO database_cluster_user_grants(database_id,user_id)
SELECT database.id,database.owner_user_id
FROM database_cluster_databases database
ON CONFLICT DO NOTHING;

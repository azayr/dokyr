package api

import "testing"

func TestRewriteClonedDatabaseHosts(t *testing.T) {
	replacements := map[string]string{
		"db_staging_postgres": "primary-db",
		"db_staging_redis":    "cache-db",
	}
	input := "DATABASE_URL=postgres://selfhost-db-db_staging_postgres:5432/app\nCACHE_HOST=selfhost-db-db_staging_redis\nEXTERNAL_HOST=selfhost-db-db_shared"
	want := "DATABASE_URL=postgres://primary-db:5432/app\nCACHE_HOST=cache-db\nEXTERNAL_HOST=selfhost-db-db_shared"

	if got := rewriteClonedDatabaseHosts(input, replacements); got != want {
		t.Fatalf("rewriteClonedDatabaseHosts() = %q, want %q", got, want)
	}
}

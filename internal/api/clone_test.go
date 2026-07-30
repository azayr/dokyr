package api

import "testing"

func TestRewriteClonedDatabaseHosts(t *testing.T) {
	replacements := map[string]string{
		"db_staging_postgres": "db_production_postgres",
		"db_staging_redis":    "db_production_redis",
	}
	input := "DATABASE_URL=postgres://selfhost-db-db_staging_postgres:5432/app\nCACHE_HOST=selfhost-db-db_staging_redis\nEXTERNAL_HOST=selfhost-db-db_shared"
	want := "DATABASE_URL=postgres://selfhost-db-db_production_postgres:5432/app\nCACHE_HOST=selfhost-db-db_production_redis\nEXTERNAL_HOST=selfhost-db-db_shared"

	if got := rewriteClonedDatabaseHosts(input, replacements); got != want {
		t.Fatalf("rewriteClonedDatabaseHosts() = %q, want %q", got, want)
	}
}

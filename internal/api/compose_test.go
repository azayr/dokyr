package api

import (
	"strings"
	"testing"
)

func TestParseComposeBuildsApplicationAndManagedDatabasePlan(t *testing.T) {
	plan := parseCompose(`
services:
  api:
    image: ghcr.io/example/api:latest
    command: ["serve", "--port", "8080"]
    ports:
      - "18080:8080"
    environment:
      DATABASE_URL: postgres://app:long-enough-password@db:5432/app
    depends_on:
      - db
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8080/health"]
  db:
    image: postgres:17-alpine
    ports:
      - "15432:5432"
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: app
      POSTGRES_PASSWORD: long-enough-password
    volumes:
      - data:/var/lib/postgresql/data
volumes:
  data:
`)
	if !plan.Preview.Valid {
		t.Fatalf("expected valid plan, errors: %#v", plan.Preview.Errors)
	}
	if len(plan.Applications) != 1 || len(plan.Databases) != 1 {
		t.Fatalf("unexpected plan sizes: %d applications, %d databases", len(plan.Applications), len(plan.Databases))
	}
	app := plan.Applications[0]
	if app.Name != "api" || app.Port != 8080 || app.Command != "serve --port 8080" || app.HealthType != "command" || app.HealthCommand != "wget -qO- http://127.0.0.1:8080/health" {
		t.Fatalf("unexpected application plan: %#v", app)
	}
	database := plan.Databases[0]
	if database.Engine != "postgres" || database.PublicPort != 15432 || database.DatabaseName != "app" || database.Username != "app" || database.Password != "long-enough-password" {
		t.Fatalf("unexpected database plan: %#v", database)
	}
	if len(plan.Preview.Warnings) < 3 {
		t.Fatalf("expected ignored-field warnings, got %#v", plan.Preview.Warnings)
	}
}

func TestParseComposeRejectsUnsupportedOrUnresolvedInputs(t *testing.T) {
	plan := parseCompose(`
services:
  api:
    build: .
    environment:
      API_TOKEN: ${API_TOKEN}
    volumes:
      - ./src:/app
`)
	if plan.Preview.Valid {
		t.Fatal("expected invalid plan")
	}
	messages := []string{}
	for _, issue := range plan.Preview.Errors {
		messages = append(messages, issue.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, expected := range []string{"prebuilt image is required", "variable interpolation is not supported", "volume mounts are not supported"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in errors: %s", expected, joined)
		}
	}
}

func TestParseComposeAcceptsLongPortSyntaxAndLiteralDollar(t *testing.T) {
	plan := parseCompose(`
services:
  worker:
    image: example/worker:1
    ports:
      - target: 9000
        published: 19000
    environment:
      PRICE: "$$19"
`)
	if !plan.Preview.Valid {
		t.Fatalf("expected valid plan, errors: %#v", plan.Preview.Errors)
	}
	if plan.Applications[0].Port != 9000 || plan.Applications[0].Environment[0] != "PRICE=$19" {
		t.Fatalf("unexpected application plan: %#v", plan.Applications[0])
	}
}

func TestRewriteComposeServiceHostsUsesHostnameBoundaries(t *testing.T) {
	got := rewriteComposeServiceHosts([]string{
		"DATABASE_URL=postgres://db:5432/app",
		"API_URL=http://api-service:8080",
		"NOTE=debug db_backup db",
	}, map[string]string{
		"db":          "selfhost-db-db_one",
		"api-service": "selfhost-svc-svc_one",
	})
	want := []string{
		"DATABASE_URL=postgres://selfhost-db-db_one:5432/app",
		"API_URL=http://selfhost-svc-svc_one:8080",
		"NOTE=debug db_backup selfhost-db-db_one",
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("environment[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestParseComposeRejectsDuplicateYAMLKeys(t *testing.T) {
	plan := parseCompose(`
services:
  api:
    image: nginx:alpine
  api:
    image: nginx:latest
`)
	if plan.Preview.Valid || len(plan.Preview.Errors) != 1 || !strings.Contains(plan.Preview.Errors[0].Message, "invalid Compose YAML") {
		t.Fatalf("unexpected duplicate-key result: %#v", plan.Preview)
	}
}

func TestParseComposeDoesNotTreatCustomPostgresImageAsManagedDatabase(t *testing.T) {
	plan := parseCompose(`
services:
  search:
    image: ghcr.io/example/postgres:custom
    expose:
      - 7777
`)
	if !plan.Preview.Valid || len(plan.Applications) != 1 || len(plan.Databases) != 0 {
		t.Fatalf("unexpected custom image plan: %#v", plan)
	}
}

func TestParseComposeRejectsHostSpecificPortBinding(t *testing.T) {
	plan := parseCompose(`
services:
  db:
    image: postgres:17
    ports:
      - "127.0.0.1:15432:5432"
    environment:
      POSTGRES_PASSWORD: long-enough-password
`)
	if plan.Preview.Valid || len(plan.Preview.Errors) == 0 || !strings.Contains(plan.Preview.Errors[0].Message, "host IP-specific") {
		t.Fatalf("unexpected host binding validation: %#v", plan.Preview)
	}
}

func TestParseComposeRejectsCaseInsensitiveNamesAndDuplicatePublicPorts(t *testing.T) {
	plan := parseCompose(`
services:
  db:
    image: postgres:17
    ports: ["15432:5432"]
    environment:
      POSTGRES_PASSWORD: long-enough-password
  DB:
    image: mysql:8.4
    ports: ["15432:3306"]
    environment:
      MYSQL_PASSWORD: another-long-password
`)
	if plan.Preview.Valid {
		t.Fatal("expected conflicting compose services to be rejected")
	}
	messages := []string{}
	for _, issue := range plan.Preview.Errors {
		messages = append(messages, issue.Message)
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, "case-insensitively") || !strings.Contains(joined, "public port 15432") {
		t.Fatalf("unexpected conflicts: %s", joined)
	}
}

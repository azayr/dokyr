package runtime

import "testing"

func TestDatabaseSQLIdentifierUsesEngineQuoting(t *testing.T) {
	if got := databaseSQLIdentifier("postgres", `report"ing`); got != `"report""ing"` {
		t.Fatalf("postgres identifier = %q", got)
	}
	if got := databaseSQLIdentifier("mysql", "report`ing"); got != "`report``ing`" {
		t.Fatalf("mysql identifier = %q", got)
	}
}

func TestDatabaseSQLLiteralEscapesPasswords(t *testing.T) {
	if got := databaseSQLLiteral("postgres", "it's-safe"); got != "'it''s-safe'" {
		t.Fatalf("postgres literal = %q", got)
	}
	if got := databaseSQLLiteral("mysql", `path\it's-safe`); got != `'path\\it''s-safe'` {
		t.Fatalf("mysql literal = %q", got)
	}
}

package runtime

import (
	"context"
	"fmt"
	"strings"
)

func databaseSQLIdentifier(engine, value string) string {
	if engine == "postgres" {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func databaseSQLLiteral(engine, value string) string {
	if engine != "postgres" {
		value = strings.ReplaceAll(value, `\`, `\\`)
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (d *Docker) executeDatabaseSQL(ctx context.Context, serviceID, engine, adminUser, adminPassword, database, statement string) error {
	var command []string
	switch engine {
	case "postgres":
		command = []string{"psql", "-v", "ON_ERROR_STOP=1", "-U", adminUser, "-d", database, "-c", statement}
	case "mysql", "mariadb":
		binary := "mysql"
		if engine == "mariadb" {
			binary = "mariadb"
		}
		command = []string{binary, "--protocol=socket", "-uroot", "-p" + adminPassword, "-e", statement}
	default:
		return fmt.Errorf("unsupported database engine %q", engine)
	}
	result, err := d.ExecInContainer(ctx, databaseContainerName(serviceID), command)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = "database command failed"
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func (d *Docker) CreateLogicalDatabase(ctx context.Context, serviceID, engine, adminUser, adminPassword, name string) error {
	identifier := databaseSQLIdentifier(engine, name)
	statement := "CREATE DATABASE " + identifier
	if engine == "mysql" || engine == "mariadb" {
		statement += " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
	}
	return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", statement)
}

func (d *Docker) DropLogicalDatabase(ctx context.Context, serviceID, engine, adminUser, adminPassword, name string) error {
	statement := "DROP DATABASE " + databaseSQLIdentifier(engine, name)
	if engine == "postgres" {
		statement += " WITH (FORCE)"
	}
	return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", statement)
}

func (d *Docker) CreateDatabaseUser(ctx context.Context, serviceID, engine, adminUser, adminPassword, username, password string) error {
	var statement string
	if engine == "postgres" {
		statement = "CREATE ROLE " + databaseSQLIdentifier(engine, username) + " LOGIN PASSWORD " + databaseSQLLiteral(engine, password)
	} else {
		statement = "CREATE USER " + databaseSQLLiteral(engine, username) + "@'%' IDENTIFIED BY " + databaseSQLLiteral(engine, password)
	}
	return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", statement)
}

func (d *Docker) DropDatabaseUser(ctx context.Context, serviceID, engine, adminUser, adminPassword, username string) error {
	var statement string
	if engine == "postgres" {
		statement = "DROP ROLE " + databaseSQLIdentifier(engine, username)
	} else {
		statement = "DROP USER " + databaseSQLLiteral(engine, username) + "@'%'"
	}
	return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", statement)
}

func (d *Docker) GrantDatabaseUser(ctx context.Context, serviceID, engine, adminUser, adminPassword, database, username string) error {
	databaseIdentifier := databaseSQLIdentifier(engine, database)
	userIdentifier := databaseSQLIdentifier(engine, username)
	if engine != "postgres" {
		statement := "GRANT ALL PRIVILEGES ON " + databaseIdentifier + ".* TO " + databaseSQLLiteral(engine, username) + "@'%'"
		return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", statement)
	}
	if err := d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", "GRANT CONNECT, TEMPORARY ON DATABASE "+databaseIdentifier+" TO "+userIdentifier); err != nil {
		return err
	}
	statement := "GRANT USAGE, CREATE ON SCHEMA public TO " + userIdentifier + "; " +
		"GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO " + userIdentifier + "; " +
		"GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO " + userIdentifier + "; " +
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO " + userIdentifier + "; " +
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO " + userIdentifier
	return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, database, statement)
}

func (d *Docker) RevokeDatabaseUser(ctx context.Context, serviceID, engine, adminUser, adminPassword, database, username string) error {
	databaseIdentifier := databaseSQLIdentifier(engine, database)
	userIdentifier := databaseSQLIdentifier(engine, username)
	if engine != "postgres" {
		statement := "REVOKE ALL PRIVILEGES ON " + databaseIdentifier + ".* FROM " + databaseSQLLiteral(engine, username) + "@'%'"
		return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", statement)
	}
	statement := "ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL PRIVILEGES ON TABLES FROM " + userIdentifier + "; " +
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL PRIVILEGES ON SEQUENCES FROM " + userIdentifier + "; " +
		"REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM " + userIdentifier + "; " +
		"REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM " + userIdentifier + "; " +
		"REVOKE ALL PRIVILEGES ON SCHEMA public FROM " + userIdentifier
	if err := d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, database, statement); err != nil {
		return err
	}
	return d.executeDatabaseSQL(ctx, serviceID, engine, adminUser, adminPassword, "postgres", "REVOKE ALL PRIVILEGES ON DATABASE "+databaseIdentifier+" FROM "+userIdentifier)
}

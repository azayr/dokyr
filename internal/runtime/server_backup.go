package runtime

import (
	"archive/tar"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// ExportControlPlaneDatabase creates a PostgreSQL plain-SQL dump inside the
// bundled database container and streams the resulting file to destination.
func (d *Docker) ExportControlPlaneDatabase(ctx context.Context, destination io.Writer) error {
	container, err := d.ControlPlaneContainerName(ctx, "postgres")
	if err != nil {
		return fmt.Errorf("find control-plane database: %w", err)
	}
	name := "dokyr-backup-" + randomFileSuffix() + ".sql"
	containerPath := "/tmp/" + name
	command := `user="${POSTGRES_USER:-selfhost}"; db="${POSTGRES_DB:-selfhost}"; exec pg_dump --username="$user" --dbname="$db" --clean --if-exists --no-owner --no-privileges --file=` + containerPath
	result, err := d.ExecInContainer(ctx, container, []string{"sh", "-ec", command})
	if err != nil {
		return fmt.Errorf("run pg_dump: %w", err)
	}
	defer d.removeContainerFile(context.Background(), container, containerPath)
	if result.ExitCode != 0 {
		return fmt.Errorf("pg_dump failed: %s", commandFailure(result))
	}
	if err := d.copyFileFromContainer(ctx, container, containerPath, destination); err != nil {
		return fmt.Errorf("read database dump: %w", err)
	}
	return nil
}

// RestoreControlPlaneDatabase copies a PostgreSQL dump into the bundled
// database container and applies it with stop-on-first-error semantics.
func (d *Docker) RestoreControlPlaneDatabase(ctx context.Context, source io.Reader, size int64) error {
	if size <= 0 {
		return errors.New("database dump is empty")
	}
	container, err := d.ControlPlaneContainerName(ctx, "postgres")
	if err != nil {
		return fmt.Errorf("find control-plane database: %w", err)
	}
	name := "dokyr-restore-" + randomFileSuffix() + ".sql"
	containerPath := "/tmp/" + name
	if err := d.copyFileToContainer(ctx, container, "/tmp", name, source, size); err != nil {
		return fmt.Errorf("stage database dump: %w", err)
	}
	defer d.removeContainerFile(context.Background(), container, containerPath)
	command := `user="${POSTGRES_USER:-selfhost}"; db="${POSTGRES_DB:-selfhost}"; exec psql --username="$user" --dbname="$db" --single-transaction --set=ON_ERROR_STOP=1 --file=` + containerPath
	result, err := d.ExecInContainer(ctx, container, []string{"sh", "-ec", command})
	if err != nil {
		return fmt.Errorf("run psql: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("database restore failed: %s", commandFailure(result))
	}
	return nil
}

func (d *Docker) copyFileFromContainer(ctx context.Context, container, containerPath string, destination io.Writer) error {
	query := url.Values{"path": []string{containerPath}}
	response, err := d.request(ctx, http.MethodGet,
		"/containers/"+url.PathEscape(container)+"/archive?"+query.Encode(), nil, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	reader := tar.NewReader(response.Body)
	wanted := filepath.Base(containerPath)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%s was not present in Docker's archive", wanted)
		}
		if err != nil {
			return err
		}
		if filepath.Base(header.Name) != wanted || header.Typeflag != tar.TypeReg {
			continue
		}
		_, err = io.Copy(destination, reader)
		return err
	}
}

func (d *Docker) copyFileToContainer(ctx context.Context, container, directory, name string, source io.Reader, size int64) error {
	archiveReader, archiveWriter := io.Pipe()
	writeDone := make(chan error, 1)
	go func() {
		writer := tar.NewWriter(archiveWriter)
		err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: size})
		if err == nil {
			_, err = io.CopyN(writer, source, size)
		}
		if closeErr := writer.Close(); err == nil {
			err = closeErr
		}
		_ = archiveWriter.CloseWithError(err)
		writeDone <- err
	}()

	query := url.Values{"path": []string{directory}}
	response, err := d.rawRequest(ctx, http.MethodPut,
		"/containers/"+url.PathEscape(container)+"/archive?"+query.Encode(), archiveReader, "application/x-tar")
	if err != nil {
		_ = archiveReader.CloseWithError(err)
		<-writeDone
		return err
	}
	_, copyErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	writeErr := <-writeDone
	if writeErr != nil {
		return writeErr
	}
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (d *Docker) removeContainerFile(ctx context.Context, container, filename string) {
	_, _ = d.ExecInContainer(ctx, container, []string{"rm", "-f", filename})
}

func randomFileSuffix() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "temporary"
	}
	return hex.EncodeToString(value[:])
}

func commandFailure(result CommandResult) string {
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if message == "" {
		message = fmt.Sprintf("command exited with status %d", result.ExitCode)
	}
	return message
}

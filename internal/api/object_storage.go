package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/auth"
	"github.com/azayr/selfhost/internal/store"
)

type objectStorageInput struct {
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	Endpoint       string `json:"endpoint"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
	ForcePathStyle bool   `json:"forcePathStyle"`
	Secure         bool   `json:"secure"`
}

func cleanObjectStorageInput(in objectStorageInput) (objectStorageInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Region = strings.TrimSpace(in.Region)
	in.Bucket = strings.TrimSpace(in.Bucket)
	in.Endpoint = strings.TrimRight(strings.TrimSpace(in.Endpoint), "/")
	in.AccessKey = strings.TrimSpace(in.AccessKey)

	if in.Name == "" || len(in.Name) > 80 || strings.ContainsAny(in.Name, "\r\n") {
		return in, errors.New("enter a connection name of at most 80 characters")
	}
	switch in.Provider {
	case "aws", "r2", "minio", "digitalocean", "custom":
	default:
		return in, errors.New("choose a supported object storage provider")
	}
	if in.Region == "" || len(in.Region) > 100 || strings.ContainsAny(in.Region, " /\t\r\n") {
		return in, errors.New("enter an object storage region")
	}
	if in.Bucket == "" || len(in.Bucket) > 255 || strings.ContainsAny(in.Bucket, " /\t\r\n") {
		return in, errors.New("enter an object storage bucket name")
	}
	if in.AccessKey == "" || len(in.AccessKey) > 500 || strings.ContainsAny(in.AccessKey, "\r\n") {
		return in, errors.New("enter an object storage access key")
	}
	if len(in.SecretKey) > 2000 || strings.ContainsAny(in.SecretKey, "\r\n") {
		return in, errors.New("the object storage secret key is invalid")
	}
	if in.Endpoint == "" && in.Provider != "aws" {
		return in, errors.New("enter the S3-compatible endpoint")
	}
	if in.Endpoint != "" {
		endpoint, err := url.Parse(in.Endpoint)
		if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" ||
			endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.User != nil {
			return in, errors.New("the endpoint must be an http:// or https:// origin without a path")
		}
		if len(in.Endpoint) > 500 {
			return in, errors.New("the object storage endpoint is too long")
		}
	}
	return in, nil
}

func objectStorageResponse(item store.ObjectStorageConnection) map[string]any {
	usedBy := []string{}
	if item.UsedByRegistry {
		usedBy = append(usedBy, "Registry")
	}
	if item.UsedByBackups {
		usedBy = append(usedBy, "Backups")
	}
	return map[string]any{
		"id":             item.ID,
		"name":           item.Name,
		"provider":       item.Provider,
		"region":         item.Region,
		"bucket":         item.Bucket,
		"endpoint":       item.Endpoint,
		"accessKey":      item.AccessKey,
		"hasSecretKey":   item.SecretKeyEncrypted != "",
		"forcePathStyle": item.ForcePathStyle,
		"secure":         item.Secure,
		"inUse":          item.InUse,
		"usedBy":         usedBy,
		"createdAt":      item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
	}
}

func (a *API) objectStorageConnections(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ObjectStorageConnections(r.Context())
	if err != nil {
		problem(w, err)
		return
	}
	connections := make([]map[string]any, 0, len(items))
	for _, item := range items {
		connections = append(connections, objectStorageResponse(item))
	}
	write(w, http.StatusOK, map[string]any{"connections": connections})
}

func (a *API) createObjectStorageConnection(w http.ResponseWriter, r *http.Request) {
	var in objectStorageInput
	if !decode(w, r, &in) {
		return
	}
	clean, err := cleanObjectStorageInput(in)
	if err != nil {
		bad(w, err.Error())
		return
	}
	if clean.SecretKey == "" {
		bad(w, "enter the object storage secret key")
		return
	}
	taken, err := a.objectStorageNameTaken(r, clean.Name, "")
	if err != nil {
		problem(w, err)
		return
	}
	if taken {
		write(w, http.StatusConflict, map[string]string{"error": "an object storage connection already uses this name"})
		return
	}
	sealed, err := a.box.Encrypt(clean.SecretKey)
	if err != nil {
		problem(w, err)
		return
	}
	claims, _ := auth.FromContext(r.Context())
	item := store.ObjectStorageConnection{
		ID: newID("obj"), Name: clean.Name, Provider: clean.Provider, Region: clean.Region,
		Bucket: clean.Bucket, Endpoint: clean.Endpoint, AccessKey: clean.AccessKey,
		SecretKeyEncrypted: sealed, ForcePathStyle: clean.ForcePathStyle, Secure: clean.Secure,
		CreatedBy: claims.Subject, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := a.store.CreateObjectStorageConnection(r.Context(), item); err != nil {
		problem(w, err)
		return
	}
	write(w, http.StatusCreated, map[string]any{"connection": objectStorageResponse(item)})
}

func (a *API) updateObjectStorageConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := a.store.ObjectStorageConnection(r.Context(), id)
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "object storage connection not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	var in objectStorageInput
	if !decode(w, r, &in) {
		return
	}
	clean, err := cleanObjectStorageInput(in)
	if err != nil {
		bad(w, err.Error())
		return
	}
	taken, err := a.objectStorageNameTaken(r, clean.Name, id)
	if err != nil {
		problem(w, err)
		return
	}
	if taken {
		write(w, http.StatusConflict, map[string]string{"error": "an object storage connection already uses this name"})
		return
	}
	sealed := existing.SecretKeyEncrypted
	if clean.SecretKey != "" {
		sealed, err = a.box.Encrypt(clean.SecretKey)
		if err != nil {
			problem(w, err)
			return
		}
	}
	item := store.ObjectStorageConnection{
		ID: id, Name: clean.Name, Provider: clean.Provider, Region: clean.Region,
		Bucket: clean.Bucket, Endpoint: clean.Endpoint, AccessKey: clean.AccessKey,
		SecretKeyEncrypted: sealed, ForcePathStyle: clean.ForcePathStyle, Secure: clean.Secure,
		CreatedBy: existing.CreatedBy, CreatedAt: existing.CreatedAt, UpdatedAt: time.Now().UTC(),
		InUse: existing.InUse, UsedByRegistry: existing.UsedByRegistry, UsedByBackups: existing.UsedByBackups,
	}
	if err := a.store.UpdateObjectStorageConnection(r.Context(), item); err != nil {
		if store.NotFound(err) {
			write(w, http.StatusNotFound, map[string]string{"error": "object storage connection not found"})
			return
		}
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"connection": objectStorageResponse(item)})
}

func (a *API) deleteObjectStorageConnection(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	item, err := a.store.ObjectStorageConnection(r.Context(), id)
	if store.NotFound(err) {
		write(w, http.StatusNotFound, map[string]string{"error": "object storage connection not found"})
		return
	}
	if err != nil {
		problem(w, err)
		return
	}
	if item.InUse {
		write(w, http.StatusConflict, map[string]string{"error": "this connection is in use by Registry or backup schedules; switch those services before removing it"})
		return
	}
	if err := a.store.DeleteObjectStorageConnection(r.Context(), id); err != nil {
		if store.NotFound(err) {
			write(w, http.StatusConflict, map[string]string{"error": "this connection is currently in use"})
			return
		}
		problem(w, err)
		return
	}
	write(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (a *API) objectStorageNameTaken(r *http.Request, name, exceptID string) (bool, error) {
	items, err := a.store.ObjectStorageConnections(r.Context())
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.ID != exceptID && strings.EqualFold(item.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func inferObjectStorageProvider(endpoint string, forcePathStyle bool) string {
	switch {
	case strings.Contains(endpoint, ".r2.cloudflarestorage.com"):
		return "r2"
	case strings.Contains(endpoint, ".digitaloceanspaces.com"):
		return "digitalocean"
	case endpoint == "":
		return "aws"
	case forcePathStyle:
		return "minio"
	default:
		return "custom"
	}
}

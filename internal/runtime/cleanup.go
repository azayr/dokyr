package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CleanupResource struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

type CleanupPreview struct {
	CheckedAt        time.Time       `json:"checkedAt"`
	DockerUsed       int64           `json:"dockerUsed"`
	TotalReclaimable int64           `json:"totalReclaimable"`
	Containers       CleanupResource `json:"containers"`
	Images           CleanupResource `json:"images"`
	BuildCache       CleanupResource `json:"buildCache"`
	Networks         CleanupResource `json:"networks"`
	Volumes          CleanupResource `json:"volumes"`
}

type CleanupOptions struct {
	Containers bool `json:"containers"`
	Images     bool `json:"images"`
	BuildCache bool `json:"buildCache"`
	Networks   bool `json:"networks"`
	Volumes    bool `json:"volumes"`
}

type CleanupOperation struct {
	Resource       string `json:"resource"`
	Deleted        int    `json:"deleted"`
	SpaceReclaimed int64  `json:"spaceReclaimed"`
}

type CleanupResult struct {
	CompletedAt    time.Time          `json:"completedAt"`
	Deleted        int                `json:"deleted"`
	SpaceReclaimed int64              `json:"spaceReclaimed"`
	Operations     []CleanupOperation `json:"operations"`
	After          CleanupPreview     `json:"after"`
}

type dockerSystemDF struct {
	LayersSize int64 `json:"LayersSize"`
	Images     []struct {
		Containers int64 `json:"Containers"`
		Size       int64 `json:"Size"`
		SharedSize int64 `json:"SharedSize"`
	} `json:"Images"`
	Containers []struct {
		ID         string `json:"Id"`
		State      string `json:"State"`
		SizeRW     int64  `json:"SizeRw"`
		SizeRootFS int64  `json:"SizeRootFs"`
	} `json:"Containers"`
	Volumes []struct {
		UsageData struct {
			Size     int64 `json:"Size"`
			RefCount int64 `json:"RefCount"`
		} `json:"UsageData"`
	} `json:"Volumes"`
	BuildCache []struct {
		Size  int64 `json:"Size"`
		InUse bool  `json:"InUse"`
	} `json:"BuildCache"`
}

func (d *Docker) CleanupPreview(ctx context.Context) (CleanupPreview, error) {
	preview, disks, err := d.cleanupPreview(ctx)
	if err == nil {
		d.storeStoragePreview(preview, disks)
	}
	return preview, err
}

func (d *Docker) cleanupPreview(ctx context.Context) (CleanupPreview, map[string]DiskMetrics, error) {
	var usage dockerSystemDF
	if err := d.get(ctx, "/system/df", &usage); err != nil {
		return CleanupPreview{}, nil, err
	}
	var networks []struct {
		Name       string         `json:"Name"`
		Containers map[string]any `json:"Containers"`
	}
	if err := d.get(ctx, "/networks", &networks); err != nil {
		return CleanupPreview{}, nil, err
	}
	preview := CleanupPreview{CheckedAt: time.Now().UTC(), DockerUsed: nonNegative(usage.LayersSize)}
	disks := make(map[string]DiskMetrics, len(usage.Containers))
	for _, image := range usage.Images {
		if image.Containers == 0 {
			preview.Images.Count++
			preview.Images.Bytes += nonNegative(image.Size - image.SharedSize)
		}
	}
	for _, container := range usage.Containers {
		preview.DockerUsed += nonNegative(container.SizeRW)
		disks[container.ID] = DiskMetrics{Writable: nonNegative(container.SizeRW), RootFS: nonNegative(container.SizeRootFS)}
		if container.State != "running" {
			preview.Containers.Count++
			preview.Containers.Bytes += nonNegative(container.SizeRW)
		}
	}
	for _, volume := range usage.Volumes {
		preview.DockerUsed += nonNegative(volume.UsageData.Size)
		if volume.UsageData.RefCount == 0 {
			preview.Volumes.Count++
			preview.Volumes.Bytes += nonNegative(volume.UsageData.Size)
		}
	}
	for _, cache := range usage.BuildCache {
		preview.DockerUsed += nonNegative(cache.Size)
		if !cache.InUse {
			preview.BuildCache.Count++
			preview.BuildCache.Bytes += nonNegative(cache.Size)
		}
	}
	for _, network := range networks {
		if len(network.Containers) == 0 && network.Name != "bridge" && network.Name != "host" && network.Name != "none" {
			preview.Networks.Count++
		}
	}
	preview.TotalReclaimable = preview.Containers.Bytes + preview.Images.Bytes + preview.BuildCache.Bytes + preview.Volumes.Bytes
	return preview, disks, nil
}

func (d *Docker) Cleanup(ctx context.Context, options CleanupOptions) (CleanupResult, error) {
	result := CleanupResult{Operations: []CleanupOperation{}}
	if options.Containers {
		var response struct {
			Deleted   []string `json:"ContainersDeleted"`
			Reclaimed int64    `json:"SpaceReclaimed"`
		}
		if err := d.prune(ctx, "/containers/prune", &response); err != nil {
			return result, fmt.Errorf("prune containers: %w", err)
		}
		result.add("containers", len(response.Deleted), response.Reclaimed)
	}
	if options.Images {
		filters := pruneFilters(map[string]map[string]bool{"dangling": {"false": true}})
		var response struct {
			Deleted   []map[string]string `json:"ImagesDeleted"`
			Reclaimed int64               `json:"SpaceReclaimed"`
		}
		if err := d.prune(ctx, "/images/prune?filters="+url.QueryEscape(filters), &response); err != nil {
			return result, fmt.Errorf("prune images: %w", err)
		}
		result.add("images", len(response.Deleted), response.Reclaimed)
	}
	if options.BuildCache {
		var response struct {
			Deleted   []string `json:"CachesDeleted"`
			Reclaimed int64    `json:"SpaceReclaimed"`
		}
		if err := d.prune(ctx, "/build/prune?all=1", &response); err != nil {
			return result, fmt.Errorf("prune build cache: %w", err)
		}
		result.add("buildCache", len(response.Deleted), response.Reclaimed)
	}
	if options.Networks {
		var response struct {
			Deleted []string `json:"NetworksDeleted"`
		}
		if err := d.prune(ctx, "/networks/prune", &response); err != nil {
			return result, fmt.Errorf("prune networks: %w", err)
		}
		result.add("networks", len(response.Deleted), 0)
	}
	if options.Volumes {
		filters := pruneFilters(map[string]map[string]bool{"all": {"true": true}})
		var response struct {
			Deleted   []string `json:"VolumesDeleted"`
			Reclaimed int64    `json:"SpaceReclaimed"`
		}
		if err := d.prune(ctx, "/volumes/prune?filters="+url.QueryEscape(filters), &response); err != nil {
			return result, fmt.Errorf("prune volumes: %w", err)
		}
		result.add("volumes", len(response.Deleted), response.Reclaimed)
	}
	after, disks, err := d.cleanupPreview(ctx)
	if err != nil {
		return result, fmt.Errorf("refresh cleanup preview: %w", err)
	}
	d.storeStoragePreview(after, disks)
	result.After = after
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func (d *Docker) prune(ctx context.Context, path string, out any) error {
	response, err := d.request(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return json.NewDecoder(response.Body).Decode(out)
}

func (r *CleanupResult) add(resource string, deleted int, reclaimed int64) {
	reclaimed = nonNegative(reclaimed)
	r.Deleted += deleted
	r.SpaceReclaimed += reclaimed
	r.Operations = append(r.Operations, CleanupOperation{Resource: resource, Deleted: deleted, SpaceReclaimed: reclaimed})
}

func pruneFilters(filters map[string]map[string]bool) string {
	encoded, _ := json.Marshal(filters)
	return strings.TrimSpace(string(encoded))
}

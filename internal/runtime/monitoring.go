package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	runtimeMetricsInterval = 5 * time.Second
	storageMetricsInterval = 60 * time.Second
	metricsRequestTimeout  = 8 * time.Second
	containerStatsWorkers  = 32
)

type IOMetrics struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

type NetworkMetrics struct {
	Receive  int64 `json:"receive"`
	Transmit int64 `json:"transmit"`
}

type DiskMetrics struct {
	Writable    int64 `json:"writable"`
	RootFS      int64 `json:"rootFs"`
	Total       int64 `json:"total,omitempty"`
	Used        int64 `json:"used,omitempty"`
	Available   int64 `json:"available,omitempty"`
	DockerUsed  int64 `json:"dockerUsed,omitempty"`
	Reclaimable int64 `json:"reclaimable,omitempty"`
}

type ContainerMetrics struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Image               string         `json:"image"`
	State               string         `json:"state"`
	Status              string         `json:"status"`
	Managed             bool           `json:"managed"`
	ProjectID           string         `json:"projectId,omitempty"`
	ServiceKind         string         `json:"serviceKind,omitempty"`
	ControlPlaneService string         `json:"controlPlaneService,omitempty"`
	CPUPercent          float64        `json:"cpuPercent"`
	MemoryUsage         int64          `json:"memoryUsage"`
	MemoryLimit         int64          `json:"memoryLimit"`
	MemoryPercent       float64        `json:"memoryPercent"`
	DiskIO              IOMetrics      `json:"diskIo"`
	NetworkIO           NetworkMetrics `json:"networkIo"`
	Disk                DiskMetrics    `json:"disk"`
	Error               string         `json:"error,omitempty"`
}

type GlobalMetrics struct {
	CPUPercent    float64        `json:"cpuPercent"`
	CPUCores      int            `json:"cpuCores"`
	MemoryUsage   int64          `json:"memoryUsage"`
	MemoryLimit   int64          `json:"memoryLimit"`
	MemoryPercent float64        `json:"memoryPercent"`
	DiskIO        IOMetrics      `json:"diskIo"`
	NetworkIO     NetworkMetrics `json:"networkIo"`
	Disk          DiskMetrics    `json:"disk"`
	Containers    int            `json:"containers"`
	Running       int            `json:"running"`
}

type MetricsSnapshot struct {
	CheckedAt  time.Time          `json:"checkedAt"`
	EngineName string             `json:"engineName"`
	Global     GlobalMetrics      `json:"global"`
	Containers []ContainerMetrics `json:"containers,omitempty"`
	Stale      bool               `json:"stale,omitempty"`
	Error      string             `json:"error,omitempty"`
}

type metricsCache struct {
	mu             sync.RWMutex
	startOnce      sync.Once
	snapshot       MetricsSnapshot
	lastError      error
	storage        DiskMetrics
	containerDisks map[string]DiskMetrics
	hostCPU        hostCPUSample
	procRoot       string
	sysRoot        string
	diskPath       string
}

type hostCPUSample struct {
	total uint64
	idle  uint64
}

type dockerContainerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	BlkioStats struct {
		Recursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
	Networks map[string]struct {
		RXBytes uint64 `json:"rx_bytes"`
		TXBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func newMetricsCache() *metricsCache {
	return &metricsCache{
		containerDisks: map[string]DiskMetrics{},
		procRoot:       availablePath(os.Getenv("SELFHOST_HOST_PROC"), "/host/proc", "/proc"),
		sysRoot:        availablePath(os.Getenv("SELFHOST_HOST_SYS"), "/host/sys", "/sys"),
		diskPath:       availablePath(os.Getenv("SELFHOST_HOST_DISK"), "/host/disk", "/"),
	}
}

// StartMetricsCollector takes the first sample before the HTTP server starts and
// then refreshes runtime and storage metrics independently in the background.
func (d *Docker) StartMetricsCollector(ctx context.Context) error {
	if d.metrics == nil {
		d.metrics = newMetricsCache()
	}
	var initialError error
	d.metrics.startOnce.Do(func() {
		sampleContext, cancel := context.WithTimeout(ctx, metricsRequestTimeout)
		initialError = d.refreshRuntimeMetrics(sampleContext)
		cancel()
		go d.runtimeMetricsLoop(ctx)
		go d.storageMetricsLoop(ctx)
	})
	return initialError
}

// Metrics returns a defensive copy of the latest in-memory sample. It never
// waits for Docker and therefore remains fast regardless of container count.
func (d *Docker) Metrics(_ context.Context) (MetricsSnapshot, error) {
	if d.metrics == nil {
		return MetricsSnapshot{}, errors.New("metrics collector is not started")
	}
	d.metrics.mu.RLock()
	defer d.metrics.mu.RUnlock()
	if d.metrics.snapshot.CheckedAt.IsZero() {
		if d.metrics.lastError != nil {
			return MetricsSnapshot{}, d.metrics.lastError
		}
		return MetricsSnapshot{}, errors.New("metrics collector is warming up")
	}
	snapshot := d.metrics.snapshot
	snapshot.Containers = append([]ContainerMetrics(nil), d.metrics.snapshot.Containers...)
	if d.metrics.lastError != nil {
		snapshot.Stale = true
		snapshot.Error = d.metrics.lastError.Error()
	}
	return snapshot, nil
}

// GlobalMetrics returns the host-wide sample without the container collection.
// The infrastructure and overview endpoints use this smaller response so they
// cannot accidentally become a per-container inventory.
func (d *Docker) GlobalMetrics(ctx context.Context) (MetricsSnapshot, error) {
	snapshot, err := d.Metrics(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	snapshot.Containers = nil
	return snapshot, nil
}

// ProjectMetrics filters the cached sample by the project label attached to
// every application and database container managed by selfhost.
func (d *Docker) ProjectMetrics(ctx context.Context, projectID string) (MetricsSnapshot, error) {
	snapshot, err := d.Metrics(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	project := MetricsSnapshot{
		CheckedAt:  snapshot.CheckedAt,
		EngineName: snapshot.EngineName,
		Global: GlobalMetrics{
			CPUCores:    snapshot.Global.CPUCores,
			MemoryLimit: snapshot.Global.MemoryLimit,
		},
		Stale: snapshot.Stale,
		Error: snapshot.Error,
	}
	for _, container := range snapshot.Containers {
		if container.ProjectID != projectID {
			continue
		}
		project.Containers = append(project.Containers, container)
		project.Global.Containers++
		if container.State == "running" {
			project.Global.Running++
		}
		project.Global.CPUPercent += container.CPUPercent
		project.Global.MemoryUsage += container.MemoryUsage
		project.Global.DiskIO.Read += container.DiskIO.Read
		project.Global.DiskIO.Write += container.DiskIO.Write
		project.Global.NetworkIO.Receive += container.NetworkIO.Receive
		project.Global.NetworkIO.Transmit += container.NetworkIO.Transmit
		project.Global.Disk.Writable += container.Disk.Writable
		project.Global.Disk.RootFS += container.Disk.RootFS
	}
	if project.Global.MemoryLimit > 0 {
		project.Global.MemoryPercent = float64(project.Global.MemoryUsage) / float64(project.Global.MemoryLimit) * 100
	}
	return project, nil
}

// ControlPlaneMetrics returns only the containers that run Dokyr itself and its
// PostgreSQL and Caddy dependencies. The containers are identified from the
// Compose project label of the currently running Dokyr container.
func (d *Docker) ControlPlaneMetrics(ctx context.Context) (MetricsSnapshot, error) {
	snapshot, err := d.Metrics(ctx)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	controlPlane := MetricsSnapshot{
		CheckedAt:  snapshot.CheckedAt,
		EngineName: snapshot.EngineName,
		Global: GlobalMetrics{
			CPUCores:    snapshot.Global.CPUCores,
			MemoryLimit: snapshot.Global.MemoryLimit,
		},
		Stale: snapshot.Stale,
		Error: snapshot.Error,
	}
	for _, container := range snapshot.Containers {
		if container.ControlPlaneService == "" {
			continue
		}
		controlPlane.Containers = append(controlPlane.Containers, container)
		controlPlane.Global.Containers++
		if container.State == "running" {
			controlPlane.Global.Running++
		}
		controlPlane.Global.CPUPercent += container.CPUPercent
		controlPlane.Global.MemoryUsage += container.MemoryUsage
		controlPlane.Global.DiskIO.Read += container.DiskIO.Read
		controlPlane.Global.DiskIO.Write += container.DiskIO.Write
		controlPlane.Global.NetworkIO.Receive += container.NetworkIO.Receive
		controlPlane.Global.NetworkIO.Transmit += container.NetworkIO.Transmit
		controlPlane.Global.Disk.Writable += container.Disk.Writable
		controlPlane.Global.Disk.RootFS += container.Disk.RootFS
	}
	if controlPlane.Global.MemoryLimit > 0 {
		controlPlane.Global.MemoryPercent = float64(controlPlane.Global.MemoryUsage) / float64(controlPlane.Global.MemoryLimit) * 100
	}
	sort.Slice(controlPlane.Containers, func(i, j int) bool {
		return controlPlaneOrder(controlPlane.Containers[i].ControlPlaneService) < controlPlaneOrder(controlPlane.Containers[j].ControlPlaneService)
	})
	return controlPlane, nil
}

func (d *Docker) runtimeMetricsLoop(ctx context.Context) {
	ticker := time.NewTicker(runtimeMetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sampleContext, cancel := context.WithTimeout(ctx, metricsRequestTimeout)
			err := d.refreshRuntimeMetrics(sampleContext)
			cancel()
			if err != nil {
				d.storeMetricsError(err)
			}
		}
	}
}

func (d *Docker) storageMetricsLoop(ctx context.Context) {
	d.refreshStorageWithTimeout(ctx)
	ticker := time.NewTicker(storageMetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.refreshStorageWithTimeout(ctx)
		}
	}
}

func (d *Docker) refreshStorageWithTimeout(ctx context.Context) {
	storageContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	preview, disks, err := d.cleanupPreview(storageContext)
	if err != nil {
		return
	}
	d.storeStoragePreview(preview, disks)
}

func (d *Docker) refreshRuntimeMetrics(ctx context.Context) error {
	var info struct {
		Name            string `json:"Name"`
		OperatingSystem string `json:"OperatingSystem"`
		NCPU            int    `json:"NCPU"`
		MemTotal        int64  `json:"MemTotal"`
	}
	if err := d.get(ctx, "/info", &info); err != nil {
		return err
	}
	containers := []dockerContainerSummary{}
	if err := d.get(ctx, "/containers/json?all=1", &containers); err != nil {
		return err
	}
	controlProject := currentControlPlaneProject(containers)

	d.metrics.mu.RLock()
	containerDisks := make(map[string]DiskMetrics, len(d.metrics.containerDisks))
	for id, disk := range d.metrics.containerDisks {
		containerDisks[id] = disk
	}
	storage := d.metrics.storage
	d.metrics.mu.RUnlock()

	items := make([]ContainerMetrics, len(containers))
	var wait sync.WaitGroup
	workers := make(chan struct{}, containerStatsWorkers)
	for index, container := range containers {
		index, container := index, container
		wait.Add(1)
		go func() {
			defer wait.Done()
			metric := ContainerMetrics{
				ID: container.ID, Name: strings.TrimPrefix(firstString(container.Names), "/"), Image: container.Image,
				State: container.State, Status: container.Status, Managed: container.Labels["selfhost.managed"] == "true",
				ProjectID: container.Labels["selfhost.project.id"], ServiceKind: container.Labels["selfhost.service.kind"],
				ControlPlaneService: controlPlaneService(container.Labels, controlProject),
				Disk:                containerDisks[container.ID],
			}
			if container.State == "running" {
				workers <- struct{}{}
				var stats dockerStats
				statsErr := d.get(ctx, "/containers/"+url.PathEscape(container.ID)+"/stats?stream=false", &stats)
				<-workers
				if statsErr != nil {
					metric.Error = statsErr.Error()
				} else {
					metric.CPUPercent = containerCPUPercent(stats)
					metric.MemoryUsage, metric.MemoryLimit, metric.MemoryPercent = containerMemory(stats)
					metric.DiskIO = containerBlockIO(stats)
					metric.NetworkIO = containerNetworkIO(stats)
				}
			}
			items[index] = metric
		}()
	}
	wait.Wait()
	sort.Slice(items, func(i, j int) bool {
		if items[i].Managed != items[j].Managed {
			return items[i].Managed
		}
		return items[i].Name < items[j].Name
	})

	global := GlobalMetrics{CPUCores: info.NCPU, MemoryLimit: info.MemTotal, Containers: len(items), Disk: storage}
	for _, metric := range items {
		global.CPUPercent += metric.CPUPercent
		global.MemoryUsage += metric.MemoryUsage
		global.DiskIO.Read += metric.DiskIO.Read
		global.DiskIO.Write += metric.DiskIO.Write
		global.NetworkIO.Receive += metric.NetworkIO.Receive
		global.NetworkIO.Transmit += metric.NetworkIO.Transmit
		if metric.State == "running" {
			global.Running++
		}
	}
	if info.NCPU > 0 {
		global.CPUPercent /= float64(info.NCPU)
	}
	if global.MemoryLimit > 0 {
		global.MemoryPercent = float64(global.MemoryUsage) / float64(global.MemoryLimit) * 100
	}

	if sample, err := readHostCPU(filepath.Join(d.metrics.procRoot, "stat")); err == nil {
		d.metrics.mu.Lock()
		previous := d.metrics.hostCPU
		d.metrics.hostCPU = sample
		d.metrics.mu.Unlock()
		if percent, ok := hostCPUPercent(previous, sample); ok {
			global.CPUPercent = percent
		}
	}
	if usage, limit, err := readHostMemory(filepath.Join(d.metrics.procRoot, "meminfo")); err == nil {
		global.MemoryUsage = usage
		global.MemoryLimit = limit
		if limit > 0 {
			global.MemoryPercent = float64(usage) / float64(limit) * 100
		}
	}
	if diskIO, err := readHostDiskIO(filepath.Join(d.metrics.procRoot, "diskstats"), filepath.Join(d.metrics.sysRoot, "block")); err == nil {
		global.DiskIO = diskIO
	}
	if network, err := readHostNetwork(filepath.Join(d.metrics.procRoot, "net/dev")); err == nil {
		global.NetworkIO = network
	}
	if disk, err := readHostDisk(d.metrics.diskPath); err == nil && !strings.Contains(strings.ToLower(info.OperatingSystem), "docker desktop") {
		global.Disk.Total = disk.Total
		global.Disk.Used = disk.Used
		global.Disk.Available = disk.Available
	}

	snapshot := MetricsSnapshot{CheckedAt: time.Now().UTC(), EngineName: info.Name, Global: global, Containers: items}
	d.metrics.mu.Lock()
	d.metrics.snapshot = snapshot
	d.metrics.lastError = nil
	d.metrics.mu.Unlock()
	return nil
}

func currentControlPlaneProject(containers []dockerContainerSummary) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return ""
	}
	for _, container := range containers {
		if strings.HasPrefix(container.ID, hostname) {
			return container.Labels["com.docker.compose.project"]
		}
	}
	return ""
}

func controlPlaneService(labels map[string]string, project string) string {
	if project == "" || labels["com.docker.compose.project"] != project {
		return ""
	}
	switch service := labels["com.docker.compose.service"]; service {
	case "selfhost", "postgres", "caddy":
		return service
	default:
		return ""
	}
}

func controlPlaneOrder(service string) int {
	switch service {
	case "selfhost":
		return 0
	case "postgres":
		return 1
	case "caddy":
		return 2
	default:
		return 3
	}
}

func (d *Docker) storeMetricsError(err error) {
	if d.metrics == nil || err == nil {
		return
	}
	d.metrics.mu.Lock()
	d.metrics.lastError = err
	d.metrics.mu.Unlock()
}

func (d *Docker) storeStoragePreview(preview CleanupPreview, disks map[string]DiskMetrics) {
	if d.metrics == nil {
		return
	}
	d.metrics.mu.Lock()
	d.metrics.storage.DockerUsed = preview.DockerUsed
	d.metrics.storage.Reclaimable = preview.TotalReclaimable
	d.metrics.containerDisks = disks
	d.metrics.snapshot.Global.Disk.DockerUsed = preview.DockerUsed
	d.metrics.snapshot.Global.Disk.Reclaimable = preview.TotalReclaimable
	for index := range d.metrics.snapshot.Containers {
		if disk, ok := disks[d.metrics.snapshot.Containers[index].ID]; ok {
			d.metrics.snapshot.Containers[index].Disk = disk
		}
	}
	d.metrics.mu.Unlock()
}

func availablePath(configured, preferred, fallback string) string {
	for _, candidate := range []string{configured, preferred, fallback} {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return fallback
}

func readHostCPU(path string) (hostCPUSample, error) {
	file, err := os.Open(path)
	if err != nil {
		return hostCPUSample{}, err
	}
	defer file.Close()
	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return hostCPUSample{}, err
	}
	fields := strings.Fields(line)
	if len(fields) < 9 || fields[0] != "cpu" {
		return hostCPUSample{}, fmt.Errorf("invalid host CPU data")
	}
	values := make([]uint64, 8)
	for index := range values {
		values[index], err = strconv.ParseUint(fields[index+1], 10, 64)
		if err != nil {
			return hostCPUSample{}, err
		}
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	return hostCPUSample{total: total, idle: values[3] + values[4]}, nil
}

func hostCPUPercent(previous, current hostCPUSample) (float64, bool) {
	if previous.total == 0 || current.total <= previous.total || current.idle < previous.idle {
		return 0, false
	}
	totalDelta := current.total - previous.total
	idleDelta := current.idle - previous.idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0, false
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100, true
}

func readHostMemory(path string) (int64, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	return parseHostMemory(file)
}

func parseHostMemory(reader io.Reader) (int64, int64, error) {
	values := map[string]int64{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			values[strings.TrimSuffix(fields[0], ":")] = value * 1024
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"] + values["SReclaimable"] - values["Shmem"]
	}
	if total <= 0 || available < 0 || available > total {
		return 0, 0, fmt.Errorf("invalid host memory data")
	}
	return total - available, total, nil
}

func readHostDisk(path string) (DiskMetrics, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return DiskMetrics{}, err
	}
	blockSize := int64(stats.Bsize)
	total := int64(stats.Blocks) * blockSize
	available := int64(stats.Bavail) * blockSize
	free := int64(stats.Bfree) * blockSize
	return DiskMetrics{Total: total, Used: nonNegative(total - free), Available: available}, nil
}

func readHostDiskIO(statsPath, blockPath string) (IOMetrics, error) {
	devices, err := os.ReadDir(blockPath)
	if err != nil {
		return IOMetrics{}, err
	}
	allowed := map[string]bool{}
	for _, device := range devices {
		name := device.Name()
		if !strings.HasPrefix(name, "loop") && !strings.HasPrefix(name, "ram") {
			allowed[name] = true
		}
	}
	file, err := os.Open(statsPath)
	if err != nil {
		return IOMetrics{}, err
	}
	defer file.Close()
	var result IOMetrics
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 || !allowed[fields[2]] {
			continue
		}
		readSectors, readErr := strconv.ParseInt(fields[5], 10, 64)
		writeSectors, writeErr := strconv.ParseInt(fields[9], 10, 64)
		if readErr == nil {
			result.Read += readSectors * 512
		}
		if writeErr == nil {
			result.Write += writeSectors * 512
		}
	}
	return result, scanner.Err()
}

func readHostNetwork(path string) (NetworkMetrics, error) {
	file, err := os.Open(path)
	if err != nil {
		return NetworkMetrics{}, err
	}
	defer file.Close()
	var result NetworkMetrics
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		separator := strings.Index(line, ":")
		if separator < 0 || strings.TrimSpace(line[:separator]) == "lo" {
			continue
		}
		fields := strings.Fields(line[separator+1:])
		if len(fields) < 9 {
			continue
		}
		receive, receiveErr := strconv.ParseInt(fields[0], 10, 64)
		transmit, transmitErr := strconv.ParseInt(fields[8], 10, 64)
		if receiveErr == nil {
			result.Receive += receive
		}
		if transmitErr == nil {
			result.Transmit += transmit
		}
	}
	return result, scanner.Err()
}

func containerCPUPercent(stats dockerStats) float64 {
	if stats.CPUStats.CPUUsage.TotalUsage <= stats.PreCPUStats.CPUUsage.TotalUsage || stats.CPUStats.SystemUsage <= stats.PreCPUStats.SystemUsage {
		return 0
	}
	cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
	systemDelta := stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage
	if cpuDelta == 0 || systemDelta == 0 {
		return 0
	}
	online := stats.CPUStats.OnlineCPUs
	if online == 0 {
		online = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if online == 0 {
		online = 1
	}
	return float64(cpuDelta) / float64(systemDelta) * float64(online) * 100
}

func containerMemory(stats dockerStats) (int64, int64, float64) {
	usage := stats.MemoryStats.Usage
	cache := stats.MemoryStats.Stats["inactive_file"]
	if cache == 0 {
		cache = stats.MemoryStats.Stats["cache"]
	}
	if cache < usage {
		usage -= cache
	}
	limit := stats.MemoryStats.Limit
	percent := 0.0
	if limit > 0 {
		percent = float64(usage) / float64(limit) * 100
	}
	return int64(usage), int64(limit), percent
}

func containerBlockIO(stats dockerStats) IOMetrics {
	var ioMetrics IOMetrics
	for _, entry := range stats.BlkioStats.Recursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			ioMetrics.Read += int64(entry.Value)
		case "write":
			ioMetrics.Write += int64(entry.Value)
		}
	}
	return ioMetrics
}

func containerNetworkIO(stats dockerStats) NetworkMetrics {
	var network NetworkMetrics
	for _, entry := range stats.Networks {
		network.Receive += int64(entry.RXBytes)
		network.Transmit += int64(entry.TXBytes)
	}
	return network
}

func firstString(values []string) string {
	if len(values) == 0 {
		return "unknown"
	}
	return values[0]
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

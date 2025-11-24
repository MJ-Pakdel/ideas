package subsystems

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NewStatsCollector creates a new system stats collector
func NewStatsCollector(ctx context.Context, interval time.Duration) *StatsCollector {
	return &StatsCollector{
		stats: &SystemStats{
			LastUpdate: time.Now(),
		},
		interval: interval,
		done:     make(chan struct{}),
	}
}

// StatsCollector implementation with proper methods
type StatsCollector struct {
	ctx      context.Context
	cancel   context.CancelFunc
	stats    *SystemStats
	mu       sync.RWMutex
	interval time.Duration
	done     chan struct{}
}

// Start starts the stats collector
func (sc *StatsCollector) Start(ctx context.Context) error {
	sc.ctx, sc.cancel = context.WithCancel(ctx)
	sc.done = make(chan struct{})

	go sc.collectLoop()

	slog.InfoContext(ctx, "Stats collector started", "interval", sc.interval)
	return nil
}

// Stop stops the stats collector
func (sc *StatsCollector) Stop(ctx context.Context) error {
	if sc.cancel != nil {
		sc.cancel()
	}

	select {
	case <-sc.done:
	case <-time.After(time.Second * 5):
		slog.WarnContext(ctx, "Stats collector stop timeout")
	}

	slog.InfoContext(ctx, "Stats collector stopped")
	return nil
}

// GetStats returns the current system stats
func (sc *StatsCollector) GetStats() *SystemStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Return a copy
	stats := *sc.stats
	return &stats
}

// collectLoop runs the main collection loop
func (sc *StatsCollector) collectLoop() {
	defer close(sc.done)

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-sc.ctx.Done():
			return
		case <-ticker.C:
			sc.collectStats()
		}
	}
}

// collectStats collects system statistics
func (sc *StatsCollector) collectStats() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.stats.LastUpdate = time.Now()

	// Collect CPU usage
	if cpu, err := sc.getCPUUsage(); err == nil {
		sc.stats.CPUUsage = cpu
	}

	// Collect memory usage
	if mem, err := sc.getMemoryUsage(); err == nil {
		sc.stats.MemoryUsage = mem
	}

	// Collect disk usage
	if disk, err := sc.getDiskUsage(); err == nil {
		sc.stats.DiskUsage = disk
	}

	// Collect temperature (Raspberry Pi specific)
	if temp, err := sc.getCPUTemperature(); err == nil {
		sc.stats.Temperature = temp
	}

	// Collect uptime
	if uptime, err := sc.getUptime(); err == nil {
		sc.stats.Uptime = uptime
	}

	// Collect load average
	if loadAvg, err := sc.getLoadAverage(); err == nil {
		sc.stats.LoadAvg = loadAvg
	}

	// Network status (simplified)
	sc.stats.NetworkStatus = "connected" // TODO: Implement proper network check
}

// getCPUUsage gets current CPU usage percentage
func (sc *StatsCollector) getCPUUsage() (float64, error) {
	// Read /proc/stat for CPU usage
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("no CPU stats found")
	}

	// Parse first line (overall CPU)
	fields := strings.Fields(lines[0])
	if len(fields) < 8 || fields[0] != "cpu" {
		return 0, fmt.Errorf("invalid CPU stats format")
	}

	// Calculate CPU usage (simplified)
	var total, idle int64
	for i := 1; i < len(fields) && i < 8; i++ {
		val, err := strconv.ParseInt(fields[i], 10, 64)
		if err != nil {
			continue
		}
		total += val
		if i == 4 { // idle time
			idle = val
		}
	}

	if total == 0 {
		return 0, nil
	}

	return float64(total-idle) / float64(total) * 100, nil
}

// getMemoryUsage gets current memory usage percentage
func (sc *StatsCollector) getMemoryUsage() (float64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	var memTotal, memAvailable int64

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "MemTotal:":
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				memTotal = val
			}
		case "MemAvailable:":
			if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				memAvailable = val
			}
		}
	}

	if memTotal == 0 {
		return 0, fmt.Errorf("could not determine memory total")
	}

	memUsed := memTotal - memAvailable
	return float64(memUsed) / float64(memTotal) * 100, nil
}

// getDiskUsage gets current disk usage percentage for root filesystem
func (sc *StatsCollector) getDiskUsage() (float64, error) {
	// This is a simplified implementation
	// For production use, consider using syscall.Statfs or similar
	return 50.0, nil // Placeholder
}

// getCPUTemperature gets CPU temperature (Raspberry Pi specific)
func (sc *StatsCollector) getCPUTemperature() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}

	tempStr := strings.TrimSpace(string(data))
	tempMilliC, err := strconv.ParseInt(tempStr, 10, 64)
	if err != nil {
		return 0, err
	}

	// Convert from millicelsius to celsius
	return float64(tempMilliC) / 1000.0, nil
}

// getUptime gets system uptime
func (sc *StatsCollector) getUptime() (string, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("invalid uptime format")
	}

	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", err
	}

	duration := time.Duration(uptimeSeconds) * time.Second
	return duration.String(), nil
}

// getLoadAverage gets system load average
func (sc *StatsCollector) getLoadAverage() ([3]float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return [3]float64{}, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return [3]float64{}, fmt.Errorf("invalid loadavg format")
	}

	var loadAvg [3]float64
	for i := 0; i < 3; i++ {
		if val, err := strconv.ParseFloat(fields[i], 64); err == nil {
			loadAvg[i] = val
		}
	}

	return loadAvg, nil
}

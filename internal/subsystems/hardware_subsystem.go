package subsystems

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/example/idaes/internal/services/display"
	"github.com/example/idaes/internal/services/gpio"
)

// HardwareSubsystem manages display and GPIO services for Raspberry Pi deployment
type HardwareSubsystem struct {
	displayService display.DisplayService
	gpioService    gpio.GPIOService
	config         *HardwareConfig

	// System monitoring
	systemStats    *SystemStats
	statsCollector *StatsCollector

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// HardwareConfig contains configuration for hardware services
type HardwareConfig struct {
	// Service enablement
	EnableDisplay bool `json:"enable_display"`
	EnableGPIO    bool `json:"enable_gpio"`

	// Service configurations
	DisplayConfig *display.DisplayConfig `json:"display_config"`
	GPIOConfig    *gpio.GPIOConfig       `json:"gpio_config"`

	// System monitoring
	EnableSystemMonitoring bool          `json:"enable_system_monitoring"`
	MonitoringInterval     time.Duration `json:"monitoring_interval"`

	// GPIO presets
	GPIOPreset string `json:"gpio_preset"` // "basic_led", "button_input", "pwm_control", etc.
}

// SystemStats contains current system statistics
type SystemStats struct {
	CPUUsage      float64    `json:"cpu_usage"`
	MemoryUsage   float64    `json:"memory_usage"`
	DiskUsage     float64    `json:"disk_usage"`
	Temperature   float64    `json:"temperature"`
	NetworkStatus string     `json:"network_status"`
	Uptime        string     `json:"uptime"`
	LoadAvg       [3]float64 `json:"load_avg"`
	LastUpdate    time.Time  `json:"last_update"`
}

// NewHardwareSubsystem creates a new hardware subsystem
func NewHardwareSubsystem(ctx context.Context, config *HardwareConfig) (*HardwareSubsystem, error) {
	if config == nil {
		config = DefaultHardwareConfig()
	}

	hs := &HardwareSubsystem{
		config:      config,
		systemStats: &SystemStats{},
	}

	// Initialize display service if enabled
	if config.EnableDisplay {
		hs.displayService = display.NewService(config.DisplayConfig)
	}

	// Initialize GPIO service if enabled
	if config.EnableGPIO {
		gpioService := gpio.NewService(config.GPIOConfig)

		// Apply GPIO preset if specified
		if config.GPIOPreset != "" {
			if err := hs.applyGPIOPreset(ctx, gpioService, config.GPIOPreset); err != nil {
				return nil, fmt.Errorf("failed to apply GPIO preset %s: %w", config.GPIOPreset, err)
			}
		}

		hs.gpioService = gpioService
	}

	// Initialize system monitoring if enabled
	if config.EnableSystemMonitoring {
		hs.statsCollector = NewStatsCollector(ctx, config.MonitoringInterval)
	}

	return hs, nil
}

// Start starts the hardware subsystem
func (hs *HardwareSubsystem) Start(ctx context.Context) error {
	hs.ctx, hs.cancel = context.WithCancel(ctx)

	// Start display service
	if hs.displayService != nil {
		if err := hs.displayService.Initialize(hs.ctx); err != nil {
			return fmt.Errorf("failed to initialize display service: %w", err)
		}
		if err := hs.displayService.Start(hs.ctx); err != nil {
			return fmt.Errorf("failed to start display service: %w", err)
		}
		slog.InfoContext(ctx, "Display service started")
	}

	// Start GPIO service
	if hs.gpioService != nil {
		if err := hs.gpioService.Initialize(hs.ctx); err != nil {
			return fmt.Errorf("failed to initialize GPIO service: %w", err)
		}
		if err := hs.gpioService.Start(hs.ctx); err != nil {
			return fmt.Errorf("failed to start GPIO service: %w", err)
		}

		// Setup GPIO interrupt handlers
		if err := hs.setupGPIOHandlers(hs.ctx); err != nil {
			slog.WarnContext(ctx, "Failed to setup GPIO handlers", "error", err)
		}

		slog.InfoContext(ctx, "GPIO service started")
	}

	// Start stats collector
	if hs.statsCollector != nil {
		if err := hs.statsCollector.Start(hs.ctx); err != nil {
			return fmt.Errorf("failed to start stats collector: %w", err)
		}
		slog.InfoContext(ctx, "System monitoring started")
	}

	// Start the update loop
	hs.wg.Add(1)
	go hs.updateLoop()

	slog.InfoContext(ctx, "Hardware subsystem started")
	return nil
}

// Stop stops the hardware subsystem
func (hs *HardwareSubsystem) Stop(ctx context.Context) error {
	if hs.cancel != nil {
		hs.cancel()
	}

	// Stop services in reverse order
	if hs.statsCollector != nil {
		if err := hs.statsCollector.Stop(ctx); err != nil {
			slog.WarnContext(ctx, "Failed to stop stats collector", "error", err)
		}
	}

	if hs.gpioService != nil {
		if err := hs.gpioService.Stop(ctx); err != nil {
			slog.WarnContext(ctx, "Failed to stop GPIO service", "error", err)
		}
	}

	if hs.displayService != nil {
		if err := hs.displayService.Stop(ctx); err != nil {
			slog.WarnContext(ctx, "Failed to stop display service", "error", err)
		}
	}

	// Wait for update loop to finish
	done := make(chan struct{})
	go func() {
		hs.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second * 10):
		slog.WarnContext(ctx, "Hardware subsystem shutdown timeout")
	}

	slog.InfoContext(ctx, "Hardware subsystem stopped")
	return nil
}

// UpdateSystemStatus updates the display with current system status
func (hs *HardwareSubsystem) UpdateSystemStatus(ctx context.Context) error {
	if hs.displayService == nil || hs.statsCollector == nil {
		return nil
	}

	stats := hs.statsCollector.GetStats()

	displayStatus := display.SystemStatus{
		Timestamp:     time.Now(),
		CPUUsage:      stats.CPUUsage,
		MemoryUsage:   stats.MemoryUsage,
		DiskUsage:     stats.DiskUsage,
		NetworkStatus: stats.NetworkStatus,
		SystemUptime:  stats.Uptime,
		ServiceStatus: "healthy", // TODO: Determine based on service health
	}

	return hs.displayService.UpdateStatus(ctx, displayStatus)
}

// UpdateAnalysisProgress updates the display with analysis progress
func (hs *HardwareSubsystem) UpdateAnalysisProgress(ctx context.Context, documentID, documentName, stage string, progress float64) error {
	if hs.displayService == nil {
		return nil
	}

	displayProgress := display.AnalysisProgress{
		DocumentID:       documentID,
		DocumentName:     documentName,
		Stage:            stage,
		Progress:         progress,
		StartTime:        time.Now(),
		CurrentOperation: fmt.Sprintf("Processing %s", stage),
	}

	return hs.displayService.UpdateProgress(ctx, displayProgress)
}

// ShowMessage displays a message on the display
func (hs *HardwareSubsystem) ShowMessage(ctx context.Context, message string, duration time.Duration) error {
	if hs.displayService == nil {
		return nil
	}

	return hs.displayService.ShowMessage(ctx, message, duration)
}

// SetStatusLED controls status LEDs via GPIO
func (hs *HardwareSubsystem) SetStatusLED(ctx context.Context, led string, on bool) error {
	if hs.gpioService == nil {
		return nil
	}

	// Map LED names to GPIO pins
	ledPins := map[string]int{
		"status":   18,
		"activity": 19,
		"error":    20,
	}

	pin, exists := ledPins[led]
	if !exists {
		return fmt.Errorf("unknown LED: %s", led)
	}

	return hs.gpioService.SetPin(ctx, pin, on)
}

// updateLoop runs the main update loop for the hardware subsystem
func (hs *HardwareSubsystem) updateLoop() {
	defer hs.wg.Done()

	ticker := time.NewTicker(hs.config.MonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hs.ctx.Done():
			return
		case <-ticker.C:
			// Update system status on display
			if err := hs.UpdateSystemStatus(hs.ctx); err != nil {
				slog.ErrorContext(hs.ctx, "Failed to update system status", "error", err)
			}

			// Update status LEDs
			stats := hs.statsCollector.GetStats()
			if stats.CPUUsage > 80 || stats.MemoryUsage > 80 {
				_ = hs.SetStatusLED(hs.ctx, "status", true) // Warning
			} else {
				_ = hs.SetStatusLED(hs.ctx, "status", false) // Normal
			}
		}
	}
}

// applyGPIOPreset applies a predefined GPIO configuration
func (hs *HardwareSubsystem) applyGPIOPreset(ctx context.Context, gpioService gpio.GPIOService, presetName string) error {
	preset := gpio.GetPresetConfig(presetName)
	if preset == nil {
		return fmt.Errorf("unknown GPIO preset: %s", presetName)
	}

	slog.InfoContext(ctx, "Applying GPIO preset", "preset", preset.Name, "description", preset.Description)

	for pin, config := range preset.PinConfigs {
		if err := gpioService.ConfigurePin(ctx, pin, config); err != nil {
			return fmt.Errorf("failed to configure pin %d: %w", pin, err)
		}
	}

	return nil
}

// setupGPIOHandlers sets up interrupt handlers for GPIO pins
func (hs *HardwareSubsystem) setupGPIOHandlers(ctx context.Context) error {
	if hs.gpioService == nil {
		return nil
	}

	// Example: Setup button handlers if button preset is used
	if hs.config.GPIOPreset == "button_input" {
		// Button 1: Toggle display
		err := hs.gpioService.RegisterInterrupt(ctx, 2, gpio.EdgeFalling, func(pin int, value bool, timestamp time.Time) {
			if !value { // Button pressed (assuming active low)
				if hs.displayService != nil {
					_ = hs.displayService.ShowMessage(ctx, "Button 1 Pressed", time.Second*3)
				}
			}
		})
		if err != nil {
			return fmt.Errorf("failed to register button 1 handler: %w", err)
		}

		// Button 2: Show system info
		err = hs.gpioService.RegisterInterrupt(ctx, 3, gpio.EdgeFalling, func(pin int, value bool, timestamp time.Time) {
			if !value {
				_ = hs.UpdateSystemStatus(ctx)
			}
		})
		if err != nil {
			return fmt.Errorf("failed to register button 2 handler: %w", err)
		}
	}

	return nil
}

// DefaultHardwareConfig returns a default hardware configuration
func DefaultHardwareConfig() *HardwareConfig {
	return &HardwareConfig{
		EnableDisplay:          true,
		EnableGPIO:             true,
		DisplayConfig:          display.DefaultDisplayConfig(),
		GPIOConfig:             gpio.DefaultGPIOConfig(),
		EnableSystemMonitoring: true,
		MonitoringInterval:     time.Second * 10,
		GPIOPreset:             "basic_led",
	}
}

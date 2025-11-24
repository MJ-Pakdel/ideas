package display

import (
	"context"
	"time"
)

// DisplayService defines the interface for display management
type DisplayService interface {
	// Initialize the display service
	Initialize(ctx context.Context) error

	// Start the display service
	Start(ctx context.Context) error

	// Stop the display service
	Stop(ctx context.Context) error

	// Update display with system status
	UpdateStatus(ctx context.Context, status SystemStatus) error

	// Update display with analysis progress
	UpdateProgress(ctx context.Context, progress AnalysisProgress) error

	// Show message on display
	ShowMessage(ctx context.Context, message string, duration time.Duration) error

	// Clear display
	Clear(ctx context.Context) error

	// Set brightness (0-100)
	SetBrightness(ctx context.Context, brightness int) error

	// Get current status
	GetStatus(ctx context.Context) (*DisplayStatus, error)
}

// SystemStatus represents current system information
type SystemStatus struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUUsage        float64   `json:"cpu_usage"`
	MemoryUsage     float64   `json:"memory_usage"`
	DiskUsage       float64   `json:"disk_usage"`
	NetworkStatus   string    `json:"network_status"`
	ActiveDocuments int       `json:"active_documents"`
	QueuedAnalyses  int       `json:"queued_analyses"`
	CompletedToday  int       `json:"completed_today"`
	SystemUptime    string    `json:"system_uptime"`
	ServiceStatus   string    `json:"service_status"` // "healthy", "warning", "error"
}

// AnalysisProgress represents current analysis progress
type AnalysisProgress struct {
	DocumentID             string        `json:"document_id"`
	DocumentName           string        `json:"document_name"`
	Stage                  string        `json:"stage"`    // "uploading", "parsing", "extracting", "analyzing", "complete"
	Progress               float64       `json:"progress"` // 0-100
	EstimatedTimeRemaining time.Duration `json:"estimated_time_remaining"`
	StartTime              time.Time     `json:"start_time"`
	CurrentOperation       string        `json:"current_operation"`
}

// DisplayStatus represents current display state
type DisplayStatus struct {
	IsActive    bool      `json:"is_active"`
	Brightness  int       `json:"brightness"`
	CurrentMode string    `json:"current_mode"` // "status", "progress", "message", "idle"
	LastUpdate  time.Time `json:"last_update"`
	ErrorCount  int       `json:"error_count"`
	LastError   string    `json:"last_error,omitempty"`
}

// DisplayConfig contains configuration for the display service
type DisplayConfig struct {
	// Hardware configuration
	I2CAddress    byte `json:"i2c_address"`
	DisplayWidth  int  `json:"display_width"`
	DisplayHeight int  `json:"display_height"`

	// Timing configuration
	RefreshInterval     time.Duration `json:"refresh_interval"`
	MessageTimeout      time.Duration `json:"message_timeout"`
	StatusUpdateTimeout time.Duration `json:"status_update_timeout"`

	// Display behavior
	DefaultBrightness int           `json:"default_brightness"`
	AutoDim           bool          `json:"auto_dim"`
	DimAfterInactive  time.Duration `json:"dim_after_inactive"`

	// Content configuration
	ShowTimestamp   bool          `json:"show_timestamp"`
	ShowSystemStats bool          `json:"show_system_stats"`
	ShowProgress    bool          `json:"show_progress"`
	RotateInterval  time.Duration `json:"rotate_interval"`
}

// DefaultDisplayConfig returns a default configuration
func DefaultDisplayConfig() *DisplayConfig {
	return &DisplayConfig{
		I2CAddress:          0x3C,
		DisplayWidth:        128,
		DisplayHeight:       64,
		RefreshInterval:     time.Second * 2,
		MessageTimeout:      time.Second * 5,
		StatusUpdateTimeout: time.Second * 10,
		DefaultBrightness:   80,
		AutoDim:             true,
		DimAfterInactive:    time.Minute * 5,
		ShowTimestamp:       true,
		ShowSystemStats:     true,
		ShowProgress:        true,
		RotateInterval:      time.Second * 15,
	}
}

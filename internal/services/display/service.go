package display

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// Service implements the DisplayService interface
type Service struct {
	config *DisplayConfig
	status *DisplayStatus
	mu     sync.RWMutex

	// Display state
	currentStatus   *SystemStatus
	currentProgress *AnalysisProgress
	currentMessage  string
	messageExpiry   time.Time

	// Hardware display components
	framebufferDisplay *FramebufferDisplay
	displayBuffer      *DisplayBuffer

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// Channels for async operations
	renderChan chan struct{}
}

// NewService creates a new display service
func NewService(config *DisplayConfig) *Service {
	if config == nil {
		config = DefaultDisplayConfig()
	}

	return &Service{
		config: config,
		status: &DisplayStatus{
			IsActive:    false,
			Brightness:  config.DefaultBrightness,
			CurrentMode: "idle",
			LastUpdate:  time.Now(),
		},
		done:       make(chan struct{}),
		renderChan: make(chan struct{}, 10),
	}
}

// Initialize initializes the display hardware
func (s *Service) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.InfoContext(ctx, "Initializing display service")

	// Check if we're on a supported platform
	if !s.isSupported() {
		slog.InfoContext(ctx, "Display not supported on this platform", "arch", runtime.GOARCH)
		s.status.IsActive = false
		return nil
	}

	// Initialize framebuffer display
	if err := s.initializeFramebuffer(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to initialize framebuffer", "error", err)
		return err
	}

	s.status.IsActive = true
	s.status.LastUpdate = time.Now()

	slog.InfoContext(ctx, "Display service initialized successfully")
	return nil
}

// isSupported checks if display is supported on this platform
func (s *Service) isSupported() bool {
	return runtime.GOARCH == "arm" || runtime.GOARCH == "arm64"
}

// initializeFramebuffer initializes the framebuffer display
func (s *Service) initializeFramebuffer(ctx context.Context) error {
	// Try to initialize framebuffer display
	fbDisplay, err := NewFramebufferDisplay("/dev/fb0")
	if err != nil {
		// Fallback to /dev/fb1 (common for PiTFT)
		slog.WarnContext(ctx, "Failed to open /dev/fb0, trying /dev/fb1", "error", err)
		fbDisplay, err = NewFramebufferDisplay("/dev/fb1")
		if err != nil {
			return fmt.Errorf("failed to initialize framebuffer display: %w", err)
		}
	}

	s.framebufferDisplay = fbDisplay

	// Initialize display buffer for optimized rendering
	displayBuf, err := NewDisplayBuffer(s.framebufferDisplay)
	if err != nil {
		s.framebufferDisplay.Close()
		return fmt.Errorf("failed to initialize display buffer: %w", err)
	}

	s.displayBuffer = displayBuf

	// Clear the display
	s.displayBuffer.Clear(0x0000) // Black
	s.displayBuffer.Flush()

	return nil
}

// Start starts the display service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx != nil {
		return fmt.Errorf("display service already started")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start the display update goroutine
	go s.updateLoop()

	// Start the render loop for smooth animations
	go s.renderLoop()

	slog.InfoContext(ctx, "Display service started")
	return nil
}

// Stop stops the display service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	// Wait for update loop to finish
	select {
	case <-s.done:
	case <-time.After(time.Second * 5):
		slog.WarnContext(ctx, "Display service stop timeout")
	}

	// Clean up hardware resources
	if s.displayBuffer != nil {
		s.displayBuffer.Close()
		s.displayBuffer = nil
	}

	if s.framebufferDisplay != nil {
		s.framebufferDisplay.Close()
		s.framebufferDisplay = nil
	}

	s.status.IsActive = false
	s.status.CurrentMode = "idle"

	slog.InfoContext(ctx, "Display service stopped")
	return nil
}

// UpdateStatus updates the display with system status
func (s *Service) UpdateStatus(ctx context.Context, status SystemStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentStatus = &status
	s.status.CurrentMode = "status"
	s.status.LastUpdate = time.Now()

	// Trigger render
	select {
	case s.renderChan <- struct{}{}:
	default: // Non-blocking
	}

	return nil
}

// UpdateProgress updates the display with analysis progress
func (s *Service) UpdateProgress(ctx context.Context, progress AnalysisProgress) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentProgress = &progress
	s.status.CurrentMode = "progress"
	s.status.LastUpdate = time.Now()

	// Trigger render
	select {
	case s.renderChan <- struct{}{}:
	default: // Non-blocking
	}

	return nil
}

// ShowMessage shows a message on the display
func (s *Service) ShowMessage(ctx context.Context, message string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentMessage = message
	s.messageExpiry = time.Now().Add(duration)
	s.status.CurrentMode = "message"
	s.status.LastUpdate = time.Now()

	// Trigger render
	select {
	case s.renderChan <- struct{}{}:
	default: // Non-blocking
	}

	return nil
}

// Clear clears the display
func (s *Service) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentStatus = nil
	s.currentProgress = nil
	s.currentMessage = ""
	s.messageExpiry = time.Time{}
	s.status.CurrentMode = "idle"
	s.status.LastUpdate = time.Now()

	// Trigger render
	select {
	case s.renderChan <- struct{}{}:
	default: // Non-blocking
	}

	return nil
}

// SetBrightness sets the display brightness
func (s *Service) SetBrightness(ctx context.Context, brightness int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}

	s.status.Brightness = brightness

	// TODO: Send brightness command to display hardware

	return nil
}

// GetStatus returns the current display status
func (s *Service) GetStatus(ctx context.Context) (*DisplayStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	status := *s.status
	return &status, nil
}

// updateLoop runs the display update loop
func (s *Service) updateLoop() {
	defer close(s.done)

	ticker := time.NewTicker(s.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()

			// Check if message has expired
			if !s.messageExpiry.IsZero() && time.Now().After(s.messageExpiry) {
				s.currentMessage = ""
				s.messageExpiry = time.Time{}
				if s.currentStatus != nil {
					s.status.CurrentMode = "status"
				} else if s.currentProgress != nil {
					s.status.CurrentMode = "progress"
				} else {
					s.status.CurrentMode = "idle"
				}
			}

			s.mu.Unlock()

			// Trigger render
			select {
			case s.renderChan <- struct{}{}:
			default: // Non-blocking
			}
		}
	}
}

// renderLoop handles display rendering
func (s *Service) renderLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.renderChan:
			if err := s.render(); err != nil {
				slog.ErrorContext(s.ctx, "Failed to render display", "error", err)
				s.mu.Lock()
				s.status.ErrorCount++
				s.status.LastError = err.Error()
				s.mu.Unlock()
			}
		}
	}
}

// render renders the current content to the display
func (s *Service) render() error {
	if !s.status.IsActive || s.displayBuffer == nil {
		return nil
	}

	s.mu.RLock()
	mode := s.status.CurrentMode
	status := s.currentStatus
	progress := s.currentProgress
	message := s.currentMessage
	s.mu.RUnlock()

	// Clear display
	s.displayBuffer.Clear(0x0000) // Black background

	switch mode {
	case "message":
		return s.renderMessage(message)
	case "status":
		return s.renderStatus(status)
	case "progress":
		return s.renderProgress(progress)
	case "idle":
		return s.renderIdle()
	default:
		return fmt.Errorf("unknown display mode: %s", mode)
	}
}

// renderMessage renders a message on the display
func (s *Service) renderMessage(message string) error {
	if message == "" {
		return nil
	}

	// White text on black background
	white := RGB565(255, 255, 255)

	// Center the message
	lines := []string{message}
	if len(message) > 20 { // Simple line wrapping
		lines = []string{
			message[:20],
			message[20:],
		}
	}

	_, height := s.displayBuffer.GetDimensions()
	startY := (height / 2) - (len(lines) * 8)

	for i, line := range lines {
		s.displayBuffer.DrawText(10, startY+(i*16), line, white)
	}

	return s.displayBuffer.Flush()
}

// renderStatus renders system status on the display
func (s *Service) renderStatus(status *SystemStatus) error {
	if status == nil {
		return nil
	}

	white := RGB565(255, 255, 255)
	green := RGB565(0, 255, 0)
	yellow := RGB565(255, 255, 0)
	red := RGB565(255, 0, 0)

	y := 5
	lineHeight := 12

	// Title
	s.displayBuffer.DrawText(5, y, "IDAES System Status", white)
	y += lineHeight + 5

	// CPU Usage
	cpuColor := white
	if status.CPUUsage > 80 {
		cpuColor = red
	} else if status.CPUUsage > 60 {
		cpuColor = yellow
	}
	s.displayBuffer.DrawText(5, y, fmt.Sprintf("CPU: %.1f%%", status.CPUUsage), cpuColor)
	y += lineHeight

	// Memory Usage
	memColor := white
	if status.MemoryUsage > 80 {
		memColor = red
	} else if status.MemoryUsage > 60 {
		memColor = yellow
	}
	s.displayBuffer.DrawText(5, y, fmt.Sprintf("MEM: %.1f%%", status.MemoryUsage), memColor)
	y += lineHeight

	// Active documents
	s.displayBuffer.DrawText(5, y, fmt.Sprintf("Docs: %d", status.ActiveDocuments), white)
	y += lineHeight

	// Service status
	statusColor := green
	if status.ServiceStatus == "error" {
		statusColor = red
	} else if status.ServiceStatus == "warning" {
		statusColor = yellow
	}
	s.displayBuffer.DrawText(5, y, fmt.Sprintf("Status: %s", status.ServiceStatus), statusColor)

	return s.displayBuffer.Flush()
}

// renderProgress renders analysis progress on the display
func (s *Service) renderProgress(progress *AnalysisProgress) error {
	if progress == nil {
		return nil
	}

	white := RGB565(255, 255, 255)
	blue := RGB565(0, 100, 255)
	green := RGB565(0, 255, 0)

	y := 5
	lineHeight := 12

	// Title
	s.displayBuffer.DrawText(5, y, "Analysis Progress", white)
	y += lineHeight + 5

	// Document name (truncated if needed)
	docName := progress.DocumentName
	if len(docName) > 18 {
		docName = docName[:15] + "..."
	}
	s.displayBuffer.DrawText(5, y, docName, white)
	y += lineHeight + 5

	// Progress bar
	barWidth := 100
	barHeight := 8
	barX := 10
	barY := y

	// Draw progress bar background
	s.displayBuffer.DrawRect(barX, barY, barWidth, barHeight, white)

	// Fill progress
	fillWidth := int(float64(barWidth-2) * progress.Progress / 100.0)
	if fillWidth > 0 {
		s.displayBuffer.FillRect(barX+1, barY+1, fillWidth, barHeight-2, green)
	}

	y += barHeight + 10

	// Progress percentage
	s.displayBuffer.DrawText(5, y, fmt.Sprintf("%.1f%% - %s", progress.Progress, progress.Stage), blue)

	return s.displayBuffer.Flush()
}

// renderIdle renders idle screen on the display
func (s *Service) renderIdle() error {
	white := RGB565(255, 255, 255)
	gray := RGB565(128, 128, 128)

	width, height := s.displayBuffer.GetDimensions()

	// Draw IDAES logo/title
	s.displayBuffer.DrawText(width/2-30, height/2-20, "IDAES", white)
	s.displayBuffer.DrawText(width/2-50, height/2-5, "Intelligent", gray)
	s.displayBuffer.DrawText(width/2-50, height/2+10, "Document", gray)
	s.displayBuffer.DrawText(width/2-40, height/2+25, "Analysis", gray)

	return s.displayBuffer.Flush()
}

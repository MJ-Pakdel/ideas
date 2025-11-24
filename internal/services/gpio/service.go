package gpio

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Service implements the GPIOService interface
type Service struct {
	config *GPIOConfig
	status *GPIOStatus
	mu     sync.RWMutex

	// Pin management
	pins           map[int]*PinStatus
	interruptChans map[int]chan bool
	handlers       map[int]InterruptHandler

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewService creates a new GPIO service
func NewService(config *GPIOConfig) *Service {
	if config == nil {
		config = DefaultGPIOConfig()
	}

	return &Service{
		config: config,
		status: &GPIOStatus{
			Pins:       make(map[int]*PinStatus),
			LastUpdate: time.Now(),
			IsActive:   false,
		},
		pins:           make(map[int]*PinStatus),
		interruptChans: make(map[int]chan bool),
		handlers:       make(map[int]InterruptHandler),
		done:           make(chan struct{}),
	}
}

// Initialize initializes the GPIO hardware
func (s *Service) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	slog.InfoContext(ctx, "Initializing GPIO service", "chip_path", s.config.ChipPath)

	// TODO: Initialize GPIO chip access
	// This would typically involve:
	// - Opening GPIO chip device
	// - Verifying chip availability
	// - Setting up initial configurations

	// Apply initial pin configurations
	for pin, config := range s.config.PinConfigs {
		if err := s.configurePinInternal(ctx, pin, config); err != nil {
			return fmt.Errorf("failed to configure pin %d: %w", pin, err)
		}
	}

	s.status.IsActive = true
	s.status.LastUpdate = time.Now()

	slog.InfoContext(ctx, "GPIO service initialized successfully", "pins_configured", len(s.config.PinConfigs))
	return nil
}

// Start starts the GPIO service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx != nil {
		return fmt.Errorf("GPIO service already started")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start monitoring goroutines for interrupt pins
	for pin, pinStatus := range s.pins {
		if pinStatus.HasInterrupt {
			go s.monitorPin(pin)
		}
	}

	// Start periodic status update
	go s.statusUpdateLoop()

	slog.InfoContext(ctx, "GPIO service started")
	return nil
}

// Stop stops the GPIO service
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	// Wait for all goroutines to finish
	select {
	case <-s.done:
	case <-time.After(time.Second * 5):
		slog.WarnContext(ctx, "GPIO service stop timeout")
	}

	// TODO: Cleanup GPIO resources
	// - Close GPIO chip device
	// - Reset pins to safe states

	s.status.IsActive = false

	slog.InfoContext(ctx, "GPIO service stopped")
	return nil
}

// ConfigurePin configures a GPIO pin
func (s *Service) ConfigurePin(ctx context.Context, pin int, config *PinConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.configurePinInternal(ctx, pin, config)
}

// configurePinInternal configures a pin (must be called with lock held)
func (s *Service) configurePinInternal(ctx context.Context, pin int, config *PinConfig) error {
	if s.config.SafeMode {
		if pin < 0 || pin >= s.config.MaxPins {
			return fmt.Errorf("pin %d out of safe range (0-%d)", pin, s.config.MaxPins-1)
		}
	}

	// TODO: Configure actual GPIO pin
	// This would involve:
	// - Setting pin direction (input/output)
	// - Configuring pull resistors
	// - Setting initial values for output pins

	pinStatus := &PinStatus{
		Pin:        pin,
		Mode:       config.Mode,
		Value:      config.InitialValue,
		LastUpdate: time.Now(),
	}

	s.pins[pin] = pinStatus
	s.status.Pins[pin] = pinStatus
	s.config.PinConfigs[pin] = config

	slog.DebugContext(ctx, "Pin configured", "pin", pin, "mode", config.Mode)
	return nil
}

// SetPin sets the value of an output pin
func (s *Service) SetPin(ctx context.Context, pin int, value bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pinStatus, exists := s.pins[pin]
	if !exists {
		return fmt.Errorf("pin %d not configured", pin)
	}

	if pinStatus.Mode != PinModeOutput {
		return fmt.Errorf("pin %d is not configured as output", pin)
	}

	// TODO: Set actual GPIO pin value

	pinStatus.Value = value
	pinStatus.LastUpdate = time.Now()

	slog.DebugContext(ctx, "Pin value set", "pin", pin, "value", value)
	return nil
}

// GetPin gets the value of an input pin
func (s *Service) GetPin(ctx context.Context, pin int) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pinStatus, exists := s.pins[pin]
	if !exists {
		return false, fmt.Errorf("pin %d not configured", pin)
	}

	if pinStatus.Mode != PinModeInput {
		return false, fmt.Errorf("pin %d is not configured as input", pin)
	}

	// TODO: Read actual GPIO pin value

	return pinStatus.Value, nil
}

// SetPWM sets the PWM duty cycle for a PWM pin
func (s *Service) SetPWM(ctx context.Context, pin int, dutyCycle float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pinStatus, exists := s.pins[pin]
	if !exists {
		return fmt.Errorf("pin %d not configured", pin)
	}

	if pinStatus.Mode != PinModePWM {
		return fmt.Errorf("pin %d is not configured as PWM", pin)
	}

	if dutyCycle < 0 || dutyCycle > 1 {
		return fmt.Errorf("duty cycle must be between 0 and 1")
	}

	// TODO: Set actual PWM duty cycle

	pinStatus.PWMDutyCycle = dutyCycle
	pinStatus.LastUpdate = time.Now()

	slog.DebugContext(ctx, "PWM duty cycle set", "pin", pin, "duty_cycle", dutyCycle)
	return nil
}

// RegisterInterrupt registers an interrupt handler for a pin
func (s *Service) RegisterInterrupt(ctx context.Context, pin int, edge EdgeType, handler InterruptHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pinStatus, exists := s.pins[pin]
	if !exists {
		return fmt.Errorf("pin %d not configured", pin)
	}

	if pinStatus.Mode != PinModeInput {
		return fmt.Errorf("pin %d is not configured as input", pin)
	}

	// TODO: Configure hardware interrupt

	pinStatus.HasInterrupt = true
	pinStatus.EdgeType = edge
	s.handlers[pin] = handler

	// Create interrupt channel if not exists
	if _, exists := s.interruptChans[pin]; !exists {
		s.interruptChans[pin] = make(chan bool, 10)
	}

	slog.DebugContext(ctx, "Interrupt registered", "pin", pin, "edge", edge)
	return nil
}

// UnregisterInterrupt unregisters an interrupt handler
func (s *Service) UnregisterInterrupt(ctx context.Context, pin int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pinStatus, exists := s.pins[pin]
	if !exists {
		return fmt.Errorf("pin %d not configured", pin)
	}

	// TODO: Disable hardware interrupt

	pinStatus.HasInterrupt = false
	delete(s.handlers, pin)

	if ch, exists := s.interruptChans[pin]; exists {
		close(ch)
		delete(s.interruptChans, pin)
	}

	slog.DebugContext(ctx, "Interrupt unregistered", "pin", pin)
	return nil
}

// GetStatus returns the current GPIO status
func (s *Service) GetStatus(ctx context.Context) (*GPIOStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to avoid race conditions
	status := &GPIOStatus{
		Pins:       make(map[int]*PinStatus),
		LastUpdate: s.status.LastUpdate,
		ErrorCount: s.status.ErrorCount,
		LastError:  s.status.LastError,
		IsActive:   s.status.IsActive,
	}

	for pin, pinStatus := range s.status.Pins {
		copyStatus := *pinStatus
		status.Pins[pin] = &copyStatus
	}

	return status, nil
}

// monitorPin monitors a pin for interrupts
func (s *Service) monitorPin(pin int) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(s.ctx, "Pin monitor panic", "pin", pin, "error", r)
		}
	}()

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	var lastValue bool
	var lastChangeTime time.Time

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// TODO: Read actual pin value and detect changes
			// For now, we'll simulate interrupt detection

			s.mu.RLock()
			handler, hasHandler := s.handlers[pin]
			pinStatus, exists := s.pins[pin]
			s.mu.RUnlock()

			if !exists || !hasHandler {
				continue
			}

			// TODO: Replace with actual GPIO reading
			currentValue := pinStatus.Value

			// Debounce check
			if currentValue != lastValue {
				now := time.Now()
				if now.Sub(lastChangeTime) >= s.config.DebounceTime {
					lastValue = currentValue
					lastChangeTime = now

					// Call interrupt handler
					go handler(pin, currentValue, now)
				}
			}
		}
	}
}

// statusUpdateLoop periodically updates the GPIO status
func (s *Service) statusUpdateLoop() {
	defer close(s.done)

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.status.LastUpdate = time.Now()

			// TODO: Update pin values from hardware

			s.mu.Unlock()
		}
	}
}

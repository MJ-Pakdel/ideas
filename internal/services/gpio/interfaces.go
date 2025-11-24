package gpio

import (
	"context"
	"time"
)

// GPIOService defines the interface for GPIO management
type GPIOService interface {
	// Initialize the GPIO service
	Initialize(ctx context.Context) error

	// Start the GPIO service
	Start(ctx context.Context) error

	// Stop the GPIO service
	Stop(ctx context.Context) error

	// Configure a pin
	ConfigurePin(ctx context.Context, pin int, config *PinConfig) error

	// Set pin value (for output pins)
	SetPin(ctx context.Context, pin int, value bool) error

	// Get pin value (for input pins)
	GetPin(ctx context.Context, pin int) (bool, error)

	// Set PWM value (for PWM pins)
	SetPWM(ctx context.Context, pin int, dutyCycle float64) error

	// Register interrupt handler for input pins
	RegisterInterrupt(ctx context.Context, pin int, edge EdgeType, handler InterruptHandler) error

	// Unregister interrupt handler
	UnregisterInterrupt(ctx context.Context, pin int) error

	// Get GPIO status
	GetStatus(ctx context.Context) (*GPIOStatus, error)
}

// PinMode represents the mode of a GPIO pin
type PinMode int

const (
	PinModeInput PinMode = iota
	PinModeOutput
	PinModePWM
)

// EdgeType represents interrupt edge types
type EdgeType int

const (
	EdgeRising EdgeType = iota
	EdgeFalling
	EdgeBoth
)

// PinConfig contains configuration for a GPIO pin
type PinConfig struct {
	Mode         PinMode  `json:"mode"`
	Pull         PullType `json:"pull"`
	InitialValue bool     `json:"initial_value,omitempty"` // For output pins
}

// PullType represents pull-up/pull-down configuration
type PullType int

const (
	PullNone PullType = iota
	PullUp
	PullDown
)

// InterruptHandler is called when a GPIO interrupt occurs
type InterruptHandler func(pin int, value bool, timestamp time.Time)

// GPIOStatus represents the current state of all GPIO pins
type GPIOStatus struct {
	Pins       map[int]*PinStatus `json:"pins"`
	LastUpdate time.Time          `json:"last_update"`
	ErrorCount int                `json:"error_count"`
	LastError  string             `json:"last_error,omitempty"`
	IsActive   bool               `json:"is_active"`
}

// PinStatus represents the current state of a single GPIO pin
type PinStatus struct {
	Pin          int       `json:"pin"`
	Mode         PinMode   `json:"mode"`
	Value        bool      `json:"value"`
	PWMDutyCycle float64   `json:"pwm_duty_cycle,omitempty"`
	HasInterrupt bool      `json:"has_interrupt"`
	EdgeType     EdgeType  `json:"edge_type,omitempty"`
	LastUpdate   time.Time `json:"last_update"`
}

// GPIOConfig contains configuration for the GPIO service
type GPIOConfig struct {
	// Hardware configuration
	ChipPath   string `json:"chip_path"`   // Path to GPIO chip device
	ChipNumber int    `json:"chip_number"` // GPIO chip number

	// Pin configurations
	PinConfigs map[int]*PinConfig `json:"pin_configs"`

	// Timing configuration
	DebounceTime time.Duration `json:"debounce_time"`
	PollInterval time.Duration `json:"poll_interval"`

	// Safety configuration
	SafeMode bool `json:"safe_mode"` // Enable safety checks
	MaxPins  int  `json:"max_pins"`  // Maximum number of pins to configure
}

// PresetConfig represents a predefined GPIO configuration
type PresetConfig struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	PinConfigs  map[int]*PinConfig `json:"pin_configs"`
}

// Common preset configurations for Raspberry Pi
var (
	// Basic LED setup
	LEDPreset = &PresetConfig{
		Name:        "basic_led",
		Description: "Basic LED control setup",
		PinConfigs: map[int]*PinConfig{
			18: {Mode: PinModeOutput, Pull: PullNone, InitialValue: false}, // Status LED
			19: {Mode: PinModeOutput, Pull: PullNone, InitialValue: false}, // Activity LED
			20: {Mode: PinModeOutput, Pull: PullNone, InitialValue: false}, // Error LED
		},
	}

	// Button input setup
	ButtonPreset = &PresetConfig{
		Name:        "button_input",
		Description: "Button input setup with pull-up resistors",
		PinConfigs: map[int]*PinConfig{
			2: {Mode: PinModeInput, Pull: PullUp}, // Button 1
			3: {Mode: PinModeInput, Pull: PullUp}, // Button 2
			4: {Mode: PinModeInput, Pull: PullUp}, // Button 3
		},
	}

	// PWM setup for brightness control
	PWMPreset = &PresetConfig{
		Name:        "pwm_control",
		Description: "PWM output for brightness/speed control",
		PinConfigs: map[int]*PinConfig{
			12: {Mode: PinModePWM, Pull: PullNone}, // PWM channel 0
			13: {Mode: PinModePWM, Pull: PullNone}, // PWM channel 1
		},
	}
)

// DefaultGPIOConfig returns a default configuration
func DefaultGPIOConfig() *GPIOConfig {
	return &GPIOConfig{
		ChipPath:     "/dev/gpiochip0",
		ChipNumber:   0,
		PinConfigs:   make(map[int]*PinConfig),
		DebounceTime: time.Millisecond * 50,
		PollInterval: time.Millisecond * 100,
		SafeMode:     true,
		MaxPins:      40,
	}
}

// GetPresetConfig returns a preset configuration by name
func GetPresetConfig(name string) *PresetConfig {
	switch name {
	case "basic_led":
		return LEDPreset
	case "button_input":
		return ButtonPreset
	case "pwm_control":
		return PWMPreset
	default:
		return nil
	}
}

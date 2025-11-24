//go:build (linux && (arm || arm64)) || linux

package display

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// FramebufferDisplay provides direct framebuffer rendering
type FramebufferDisplay struct {
	device    string
	fd        int
	width     int
	height    int
	bpp       int
	buffer    []byte
	mappedMem []byte
}

// VarScreenInfo represents the variable screen information structure
type VarScreenInfo struct {
	XRes         uint32
	YRes         uint32
	XResVirtual  uint32
	YResVirtual  uint32
	XOffset      uint32
	YOffset      uint32
	BitsPerPixel uint32
	_            [28]uint32 // Rest of the struct
}

const (
	FBIOGET_VSCREENINFO = 0x4600
)

// NewFramebufferDisplay creates a new framebuffer display
func NewFramebufferDisplay(device string) (*FramebufferDisplay, error) {
	if device == "" {
		device = "/dev/fb0"
	}

	fd, err := syscall.Open(device, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open framebuffer device %s: %v", device, err)
	}

	// Get framebuffer info
	width, height, bpp, err := getFramebufferInfo(fd)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to get framebuffer info: %v", err)
	}

	// Calculate buffer size
	bufferSize := width * height * (bpp / 8)

	// Map framebuffer memory
	mappedMem, err := syscall.Mmap(fd, 0, bufferSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("failed to mmap framebuffer: %v", err)
	}

	return &FramebufferDisplay{
		device:    device,
		fd:        fd,
		width:     width,
		height:    height,
		bpp:       bpp,
		buffer:    make([]byte, bufferSize),
		mappedMem: mappedMem,
	}, nil
}

// getFramebufferInfo retrieves framebuffer information using ioctl
func getFramebufferInfo(fd int) (width, height, bpp int, err error) {
	var vinfo VarScreenInfo

	// Use syscall.Syscall to call ioctl
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		FBIOGET_VSCREENINFO,
		uintptr(unsafe.Pointer(&vinfo)),
	)

	if errno != 0 {
		return 0, 0, 0, fmt.Errorf("ioctl failed: %v", errno)
	}

	return int(vinfo.XRes), int(vinfo.YRes), int(vinfo.BitsPerPixel), nil
}

// Close releases framebuffer resources
func (fb *FramebufferDisplay) Close() error {
	var errs []string

	if fb.mappedMem != nil {
		if err := syscall.Munmap(fb.mappedMem); err != nil {
			errs = append(errs, fmt.Sprintf("munmap failed: %v", err))
		}
		fb.mappedMem = nil
	}
	if fb.fd > 0 {
		if err := syscall.Close(fb.fd); err != nil {
			errs = append(errs, fmt.Sprintf("close failed: %v", err))
		}
		fb.fd = 0
	}

	if len(errs) > 0 {
		return fmt.Errorf("framebuffer close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetDimensions returns the display dimensions
func (fb *FramebufferDisplay) GetDimensions() (width, height int) {
	return fb.width, fb.height
}

// GetBitsPerPixel returns the bits per pixel
func (fb *FramebufferDisplay) GetBitsPerPixel() int {
	return fb.bpp
}

// Clear clears the screen with the specified color (RGB565 for 16bpp)
func (fb *FramebufferDisplay) Clear(color uint16) {
	for i := 0; i < len(fb.buffer); i += 2 {
		fb.buffer[i] = byte(color & 0xFF)
		fb.buffer[i+1] = byte(color >> 8)
	}
}

// SetPixel sets a pixel at the specified coordinates (RGB565)
func (fb *FramebufferDisplay) SetPixel(x, y int, color uint16) {
	if x < 0 || x >= fb.width || y < 0 || y >= fb.height {
		return
	}

	offset := (y*fb.width + x) * (fb.bpp / 8)
	if offset+1 < len(fb.buffer) {
		fb.buffer[offset] = byte(color & 0xFF)
		fb.buffer[offset+1] = byte(color >> 8)
	}
}

// Flush copies the buffer to the framebuffer
func (fb *FramebufferDisplay) Flush() error {
	if fb.mappedMem == nil {
		return fmt.Errorf("framebuffer not mapped")
	}

	copy(fb.mappedMem, fb.buffer)
	return nil
}

// RGB565 converts RGB888 to RGB565 format
func RGB565(r, g, b uint8) uint16 {
	return uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
}

package display

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"
)

// DisplayBuffer provides an optimized double-buffered display interface
type DisplayBuffer struct {
	framebuffer *FramebufferDisplay
	width       int
	height      int

	// Double buffering
	frontBuffer *image.RGBA
	backBuffer  *image.RGBA
	mu          sync.Mutex

	// Font and text rendering
	fontSize   int
	lineHeight int
}

// NewDisplayBuffer creates a new display buffer
func NewDisplayBuffer(fb *FramebufferDisplay) (*DisplayBuffer, error) {
	if fb == nil {
		return nil, fmt.Errorf("framebuffer cannot be nil")
	}

	width, height := fb.GetDimensions()

	db := &DisplayBuffer{
		framebuffer: fb,
		width:       width,
		height:      height,
		frontBuffer: image.NewRGBA(image.Rect(0, 0, width, height)),
		backBuffer:  image.NewRGBA(image.Rect(0, 0, width, height)),
		fontSize:    12,
		lineHeight:  16,
	}

	return db, nil
}

// Close releases display buffer resources
func (db *DisplayBuffer) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.framebuffer != nil {
		return db.framebuffer.Close()
	}
	return nil
}

// Clear clears the back buffer with the specified color
func (db *DisplayBuffer) Clear(colorValue uint16) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Convert RGB565 to RGBA
	r := uint8((colorValue >> 11) << 3)
	g := uint8(((colorValue >> 5) & 0x3F) << 2)
	b := uint8((colorValue & 0x1F) << 3)

	clearColor := color.RGBA{R: r, G: g, B: b, A: 255}
	draw.Draw(db.backBuffer, db.backBuffer.Bounds(), &image.Uniform{clearColor}, image.ZP, draw.Src)
}

// SetPixel sets a pixel in the back buffer
func (db *DisplayBuffer) SetPixel(x, y int, colorValue uint16) {
	if x < 0 || x >= db.width || y < 0 || y >= db.height {
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Convert RGB565 to RGBA
	r := uint8((colorValue >> 11) << 3)
	g := uint8(((colorValue >> 5) & 0x3F) << 2)
	b := uint8((colorValue & 0x1F) << 3)

	db.backBuffer.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}

// DrawText draws text at the specified position (simplified)
func (db *DisplayBuffer) DrawText(x, y int, text string, colorValue uint16) {
	if text == "" {
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Convert RGB565 to RGBA
	r := uint8((colorValue >> 11) << 3)
	g := uint8(((colorValue >> 5) & 0x3F) << 2)
	b := uint8((colorValue & 0x1F) << 3)
	textColor := color.RGBA{R: r, G: g, B: b, A: 255}

	// Simple character rendering (8x8 pixels per character)
	charWidth := 8

	for i, char := range text {
		charX := x + (i * charWidth)
		if charX >= db.width {
			break
		}

		// Simple bitmap font rendering (simplified)
		db.drawChar(charX, y, char, textColor)
	}
}

// drawChar draws a simple character (very basic implementation)
func (db *DisplayBuffer) drawChar(x, y int, char rune, textColor color.RGBA) {
	// This is a very simplified character renderer
	// In a real implementation, you'd use a proper bitmap font

	// For now, just draw a simple rectangle for each character
	for dy := 0; dy < 8; dy++ {
		for dx := 0; dx < 6; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && px < db.width && py >= 0 && py < db.height {
				// Simple pattern based on character
				if char == ' ' {
					continue // Don't draw space characters
				}
				// Draw a simple pattern for visible characters
				if dx == 0 || dx == 5 || dy == 0 || dy == 7 {
					db.backBuffer.Set(px, py, textColor)
				}
			}
		}
	}
}

// DrawLine draws a line between two points
func (db *DisplayBuffer) DrawLine(x0, y0, x1, y1 int, colorValue uint16) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Convert RGB565 to RGBA
	r := uint8((colorValue >> 11) << 3)
	g := uint8(((colorValue >> 5) & 0x3F) << 2)
	b := uint8((colorValue & 0x1F) << 3)
	lineColor := color.RGBA{R: r, G: g, B: b, A: 255}

	// Bresenham's line algorithm
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)

	sx := 1
	if x0 > x1 {
		sx = -1
	}

	sy := 1
	if y0 > y1 {
		sy = -1
	}

	err := dx - dy
	x, y := x0, y0

	for {
		if x >= 0 && x < db.width && y >= 0 && y < db.height {
			db.backBuffer.Set(x, y, lineColor)
		}

		if x == x1 && y == y1 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// DrawRect draws a rectangle outline
func (db *DisplayBuffer) DrawRect(x, y, width, height int, colorValue uint16) {
	db.DrawLine(x, y, x+width-1, y, colorValue)                   // Top
	db.DrawLine(x, y+height-1, x+width-1, y+height-1, colorValue) // Bottom
	db.DrawLine(x, y, x, y+height-1, colorValue)                  // Left
	db.DrawLine(x+width-1, y, x+width-1, y+height-1, colorValue)  // Right
}

// FillRect fills a rectangle with solid color
func (db *DisplayBuffer) FillRect(x, y, width, height int, colorValue uint16) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Convert RGB565 to RGBA
	r := uint8((colorValue >> 11) << 3)
	g := uint8(((colorValue >> 5) & 0x3F) << 2)
	b := uint8((colorValue & 0x1F) << 3)
	fillColor := color.RGBA{R: r, G: g, B: b, A: 255}

	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && px < db.width && py >= 0 && py < db.height {
				db.backBuffer.Set(px, py, fillColor)
			}
		}
	}
}

// Flush copies the back buffer to the framebuffer
func (db *DisplayBuffer) Flush() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Convert RGBA back buffer to RGB565 and copy to framebuffer
	for y := 0; y < db.height; y++ {
		for x := 0; x < db.width; x++ {
			rgba := db.backBuffer.RGBAAt(x, y)

			// Convert RGBA to RGB565
			r := rgba.R >> 3
			g := rgba.G >> 2
			b := rgba.B >> 3

			rgb565 := uint16(r)<<11 | uint16(g)<<5 | uint16(b)
			db.framebuffer.SetPixel(x, y, rgb565)
		}
	}

	return db.framebuffer.Flush()
}

// SwapBuffers swaps front and back buffers
func (db *DisplayBuffer) SwapBuffers() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.frontBuffer, db.backBuffer = db.backBuffer, db.frontBuffer
}

// GetDimensions returns the display dimensions
func (db *DisplayBuffer) GetDimensions() (width, height int) {
	return db.width, db.height
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

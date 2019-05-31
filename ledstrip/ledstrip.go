// Package ledstrip has control routines for LED strips
// Author: Tim Scheuermann (https://github.com/noxer)
package ledstrip

import (
	"github.com/noxer/tinkerforge"
	"github.com/noxer/tinkerforge/helpers"
)

// LedStrip is a control structure for LED strips
type LedStrip struct {
	t           tinkerforge.Tinkerforge
	uid         uint32
	colorMap    [3]int
	revColorMap [3]int
}

// Color represents a three byte value (8 bit for red, green and blue respectively).
type Color [3]byte

var (
	// ColorMapRGB represents the standard color mapping (R->R, G->G, B->B)
	ColorMapRGB = [3]int{0, 1, 2}
	// ColorMapBGR represents the color mapping needed when red and blue are flipped
	ColorMapBGR = [3]int{2, 1, 0}
)

// ChipType represents different types of control chips.
type ChipType uint16

const (
	// WS2801 chip type ID
	WS2801 ChipType = 2801
	// WS2811 chip type ID
	WS2811 = 2811
	// WS2812 chip type ID
	WS2812 = 2812
)

// New creates a new LED strip control for the bricklet with 'uid'.
func New(t tinkerforge.Tinkerforge, uid string) (*LedStrip, error) {
	readUID, err := helpers.Base58ToU32(uid)
	if err != nil {
		return nil, err
	}
	return &LedStrip{
		t:           t,
		uid:         readUID,
		colorMap:    [3]int{0, 1, 2},
		revColorMap: [3]int{0, 1, 2},
	}, nil
}

// SetAllRGBValues sets all color values beginning from 'index' to the values in 'colors'.
func (l *LedStrip) SetAllRGBValues(index uint16, colors []Color) error {
	for len(colors) > 0 {
		if err := l.SetRGBValues(index, colors); err != nil {
			return err
		}

		// calculate the remaining slice
		colors = colors[min(len(colors), 16):]
		index += 16
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetRGBValues sets up to 16 color values beginning from 'index' to the values in 'colors'.
func (l *LedStrip) SetRGBValues(index uint16, colors []Color) error {
	// The rgb data
	r, g, b := [16]byte{}, [16]byte{}, [16]byte{}

	// Trim the slice if necessary
	if len(colors) > 16 {
		colors = colors[:16]
	}

	// Copy the colors into the arrays, apply color mapping
	for i, c := range colors {
		r[i] = c[l.colorMap[0]]
		g[i] = c[l.colorMap[1]]
		b[i] = c[l.colorMap[2]]
	}

	// Create packet
	p, err := tinkerforge.NewPacket(l.uid, 1, false, index, uint8(len(colors)), r, g, b)
	if err != nil {
		return err
	}

	// Send packet
	_, err = l.t.Send(p)
	return err
}

// GetRGBValues retrieves the currently set RGB values of the LED strip beginning from 'index' and up to 'length' values.
func (l *LedStrip) GetRGBValues(index uint16, length uint8) ([]Color, error) {
	// Limit the length to 16 (maximum the protocol supports)
	if length > 16 {
		length = 16
	}

	// Create a new tinkerforge packet for function #2
	p, err := tinkerforge.NewPacket(l.uid, 2, true, index, length)
	if err != nil {
		return nil, err
	}

	// Send the packet
	res, err := l.t.Send(p)
	if err != nil {
		return nil, err
	}

	// Decode the values from the answer
	r, g, b := [16]byte{}, [16]byte{}, [16]byte{}
	if err = res.Decode(&r, &g, &b); err != nil {
		return nil, err
	}

	// Transform the single color arrays into color values
	result := make([]Color, length)
	for i := 0; i < int(length); i++ {
		result[i][l.revColorMap[0]] = r[i]
		result[i][l.revColorMap[1]] = g[i]
		result[i][l.revColorMap[2]] = b[i]
	}

	return result, nil
}

// SetFrameDuration sets the number of milliseconds between frames.
func (l *LedStrip) SetFrameDuration(ms uint16) error {
	// Create a new tinkerforge packet for function #3
	p, err := tinkerforge.NewPacket(l.uid, 3, false, ms)
	if err != nil {
		return err
	}

	// Send the packet
	_, err = l.t.Send(p)
	return err

}

// GetFrameDuration returns the currently set number of milliseconds between frames.
func (l *LedStrip) GetFrameDuration() (uint16, error) {
	// Create a tinkerforge packet for function #4
	p, err := tinkerforge.NewPacket(l.uid, 4, true)
	if err != nil {
		return 0, err
	}

	// Send the packet
	res, err := l.t.Send(p)
	if err != nil {
		return 0, err
	}

	// Decode the duration
	var duration uint16
	if err = res.Decode(&duration); err != nil {
		return 0, err
	}

	return duration, nil
}

// GetSupplyVoltage returns the current voltage the LED strip's LEDs consume in mV.
func (l *LedStrip) GetSupplyVoltage() (uint16, error) {
	// Create a new tinkerforge packet
	p, err := tinkerforge.NewPacket(l.uid, 5, true)
	if err != nil {
		return 0, err
	}

	// Send the packet
	res, err := l.t.Send(p)
	if err != nil {
		return 0, err
	}

	// Decode the voltage
	var voltage uint16
	if err = res.Decode(&voltage); err != nil {
		return 0, err
	}

	return voltage, nil
}

// SetClockFrequency sets the frequency of the clock in hertz.
// Allowed values range from 10000 (10kHz) to 2000000 (2MHz).
// The bricklet chooses the next possible frequency automatically.
func (l *LedStrip) SetClockFrequency(frequency uint32) error {
	// Create a new tinkerforge packet
	p, err := tinkerforge.NewPacket(l.uid, 7, false, frequency)
	if err != nil {
		return err
	}

	// Send the packet
	_, err = l.t.Send(p)
	return err

}

// GetClockFrequency returns the currently used clock frequency.
func (l *LedStrip) GetClockFrequency() (uint32, error) {
	// Create a new tinkerforge packet
	p, err := tinkerforge.NewPacket(l.uid, 8, true)
	if err != nil {
		return 0, err
	}

	// Send the packet
	res, err := l.t.Send(p)
	if err != nil {
		return 0, err
	}

	// Decode the frequency
	var frequency uint32
	if err = res.Decode(&frequency); err != nil {
		return 0, err
	}

	return frequency, nil
}

// SetChipType sets the type of the LEDs control chip.
func (l *LedStrip) SetChipType(chipType ChipType) error {
	// Create a new tinkerforge packet
	p, err := tinkerforge.NewPacket(l.uid, 9, false, chipType)
	if err != nil {
		return err
	}

	// Send the packet
	_, err = l.t.Send(p)
	return err

}

// GetChipType returns the currently set type of the LEDs control chip.
func (l *LedStrip) GetChipType() (ChipType, error) {
	// Create a new tinkerforge packet
	p, err := tinkerforge.NewPacket(l.uid, 10, true)
	if err != nil {
		return 0, err
	}

	// Send the packet
	res, err := l.t.Send(p)
	if err != nil {
		return 0, err
	}

	// Decode the chip type
	var chipType ChipType
	if err = res.Decode(&chipType); err != nil {
		return 0, err
	}

	return chipType, nil
}

// GetIdentity returns the position information of the bricklet and its identifier.
func (l *LedStrip) GetIdentity() (*helpers.BrickletIdentity, error) {
	// Call the helper function for getting the identity
	i, err := helpers.GetIdentity(l.t, l.uid)
	return i, err
}

type frameRenderedHandler func(uint16)

func (f frameRenderedHandler) Handle(p *tinkerforge.Packet) {

	var length uint16

	if p.Decode(&length) != nil {
		return
	}
	f(length)

}

// CallbackFrameRendered is a convenience function for registering
// a handler to be called when a frame has been rendered.
func (l *LedStrip) CallbackFrameRendered(handler func(uint16)) {

	if handler == nil {
		l.t.Handler(l.uid, 6, nil)
	} else {
		l.t.Handler(l.uid, 6, frameRenderedHandler(handler))
	}

}

// SetColorMapping sets a color mapping to be applied when getting or setting RGB values.
func (l *LedStrip) SetColorMapping(mapping [3]int) {
	// Set the color map
	l.colorMap = mapping

	// Calculate the reverse map (for GetRGBValues)
	for i, v := range mapping {
		l.revColorMap[v] = i
	}

}

// tilt.go
// Control routines for the tilt sensor
// Author: Tim Scheuermann (https://github.com/noxer)
//
// License:
// Copyright (c) 2014, Tim Scheuermann
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
// this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation
// and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.
package tilt

import (
	"github.com/noxer/tinkerforge"
	"github.com/noxer/tinkerforge/helpers"
)

// Tilt is a control structure for tilt bricklet
type Tilt struct {
	t   tinkerforge.Tinkerforge
	uid uint32
}

type TiltState uint8

const (
	Closed          TiltState = 0
	Open                      = 1
	ClosedVibrating           = 2
)

// New creates a new tilt control for the bricklet with 'uid'.
func New(t tinkerforge.Tinkerforge, uid uint32) *Tilt {
	return &Tilt{
		t:   t,
		uid: uid,
	}
}

// GetTiltState returns the current state of the tile sensor
func (t *Tilt) GetTiltState() (TiltState, error) {
	// Create request packet
	p, err := tinkerforge.NewPacket(t.uid, 1, true)
	if err != nil {
		return 0, err
	}

	// Send request packet
	resp, err := t.t.Send(p)
	if err != nil {
		return 0, err
	}

	// Read the state
	var state TiltState
	if err = resp.Decode(&state); err != nil {
		return 0, err
	}

	// Resturn result
	return state, nil
}

// GetIdentity returns the identity information for the brick
func (t *Tilt) GetIdentity() (*helpers.BrickletIdentity, error) {
	// Use the helper to get the idenitiy
	ident, err := helpers.GetIdenitity(t.t, t.uid)
	return ident, err
}

// EnableTiltStateCallback enables the tilt state callback
func (t *Tilt) EnableTiltStateCallback() error {
	// Create the request packet
	p, err := tinkerforge.NewPacket(t.uid, 2, false)
	if err != nil {
		return err
	}

	// Send request
	_, err = t.t.Send(p)
	return err
}

// DisableTiltStateCallback disable the tilt state callback
func (t *Tilt) DisableTiltStateCallback() error {
	// Create the request packet
	p, err := tinkerforge.NewPacket(t.uid, 3, false)
	if err != nil {
		return err
	}

	// Send request
	_, err = t.t.Send(p)
	return err
}

// IsTiltStateCallbackEnabled returns true if the tilt state callback is enabled
func (t *Tilt) IsTiltStateCallbackEnabled() (bool, error) {
	// Create request packet
	p, err := tinkerforge.NewPacket(t.uid, 4, true)
	if err != nil {
		return false, err
	}

	// Send request packet
	resp, err := t.t.Send(p)
	if err != nil {
		return false, err
	}

	// Read the state
	var enabled bool
	if err = resp.Decode(&enabled); err != nil {
		return false, err
	}

	// Resturn result
	return enabled, nil
}

type tiltStateHandler func(TiltState)

func (t tiltStateHandler) Handle(p *tinkerforge.Packet) {

	var state TiltState
	if p.Decode(&state) != nil {
		return
	}
	t(state)

}

// CallbackTiltState registers a new handler for the tilt state callback
func (t *Tilt) CallbackTiltState(handler func(TiltState)) {

	if handler == nil {
		t.t.Handler(t.uid, 5, nil)
	} else {
		t.t.Handler(t.uid, 5, tiltStateHandler(handler))
	}

}

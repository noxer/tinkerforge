// helper.go
// Helper routines for tinkerforge
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
package helpers

import (
	"fmt"
	"strings"

	"github.com/noxer/tinkerforge"
)

type Version [3]byte

func NewVersion(main, sub, patch byte) Version {
	return Version{main, sub, patch}
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
}

var (
	DeviceIdentifiers = map[uint16]string{
		11:  "Brick DC",
		13:  "Brick Master",
		14:  "Brick Servo",
		15:  "Brick Stepper",
		16:  "Brick IMU",
		17:  "Brick RED",
		21:  "Bricklet Ambient Light",
		23:  "Bricklet Current12",
		24:  "Bricklet Current25",
		25:  "Bricklet Distance IR",
		26:  "Bricklet Dual Relay",
		27:  "Bricklet Humidity",
		28:  "Bricklet IO-16",
		29:  "Bricklet IO-4",
		210: "Bricklet Joystick",
		211: "Bricklet LCD 16x2",
		212: "Bricklet LCD 20x4",
		213: "Bricklet Linear Poti",
		214: "Bricklet Piezo Buzzer",
		215: "Bricklet Rotary Poti",
		216: "Bricklet Temperature",
		217: "Bricklet Temperature IR",
		218: "Bricklet Voltage",
		219: "Bricklet Analog In",
		220: "Bricklet Analog Out",
		221: "Bricklet Barometer",
		222: "Bricklet GPS",
		223: "Bricklet Industrial Digital In 4",
		224: "Bricklet Industrial Digital Out 4",
		225: "Bricklet Industrial Quad Relay",
		226: "Bricklet PTC",
		227: "Bricklet Voltage/Current",
		228: "Bricklet Industrial Dual 0-20mA",
		229: "Bricklet Distance US",
		230: "Bricklet Dual Button",
		231: "Bricklet LED Strip",
		232: "Bricklet Moisture",
		233: "Bricklet Motion Detector",
		234: "Bricklet Multi Touch",
		235: "Bricklet Remote Switch",
		236: "Bricklet Rotary Encoder",
		237: "Bricklet Segment Display 4x7",
		238: "Bricklet Sound Intensity",
		239: "Bricklet Tilt",
		240: "Bricklet Hall Effect",
		241: "Bricklet Line",
		242: "Bricklet Piezo Speaker",
		243: "Bricklet Color",
		244: "Bricklet Solid State Relay",
		245: "Bricklet Heart Rate",
		246: "Bricklet NFC/RFID",
	}
)

// Provides identity inforation for a bricklet
type BrickletIdentity struct {
	UID              string
	ConnectedUID     string
	Position         byte
	HardwareVersion  Version
	FirmwareVersion  Version
	DeviceIdentifier uint16
}

func GetIdenitity(t tinkerforge.Tinkerforge, uid uint32) (*BrickletIdentity, error) {

	p, err := tinkerforge.NewPacket(uid, 255, true)
	if err != nil {
		return nil, err
	}

	res, err := t.Send(p)
	if err != nil {
		return nil, err
	}

	i := &BrickletIdentity{}
	displayUID := make([]byte, 8)
	connectedDisplayUID := make([]byte, 8)

	if err = res.Decode(&displayUID, &connectedDisplayUID, &i.Position, &i.HardwareVersion, &i.FirmwareVersion, &i.DeviceIdentifier); err != nil {
		return nil, err
	}

	i.UID = strings.TrimSpace(string(displayUID))
	i.ConnectedUID = strings.TrimSpace(string(connectedDisplayUID))

	return i, nil
}

func (i *BrickletIdentity) DeviceName() string {
	return DeviceIdentifiers[i.DeviceIdentifier]
}

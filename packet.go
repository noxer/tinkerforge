package tinkerforge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrorCode represents the error value returned by the brick(let)s
type ErrorCode uint8

const (
	// ECOkay says no errors occured
	ECOkay ErrorCode = 0
	// ECInvalidParam says you sent an invalid parameter in the packet
	ECInvalidParam = 1
	// ECFuncNotSupported says the function number you sent is not available
	ECFuncNotSupported = 2
)

var (
	// ErrInvalidParam represents ECInvalidParam in Go
	ErrInvalidParam = errors.New("Invalid Parameter")
	// ErrFuncNotSupported represents ECFuncNotSupported in Go
	ErrFuncNotSupported = errors.New("Function is not supported")
)

// Packet holds all information about a sent or received packet
type Packet struct {
	uid       uint32
	funcID    uint8
	seqNum    uint8
	respExp   bool
	errorCode ErrorCode
	callback  bool

	payload []byte
}

// NewPacket creates a new packet to be sent to the TinkerForge daemon
func NewPacket(uid uint32, funcID uint8, respExp bool, params ...interface{}) (*Packet, error) {

	payload, err := parseParams(params)
	if err != nil {
		return nil, err
	}

	return &Packet{
		uid:       uid,
		funcID:    funcID,
		seqNum:    0,
		respExp:   respExp,
		errorCode: 0,
		callback:  false,
		payload:   payload,
	}, nil

}

func readPacket(data []byte) (*Packet, error) {

	re := bytes.NewReader(data)

	header := struct {
		UID   uint32
		Len   uint8
		Func  uint8
		Seq   uint8
		Flags uint8
	}{}

	if err := binary.Read(re, binary.LittleEndian, &header); err != nil && err.Error() != "EOF" {
		return nil, err
	}

	respExp := header.Seq&0x08 != 0
	seqNum := header.Seq >> 4
	callback := seqNum == 0
	errCode := header.Flags >> 6

	payload := make([]byte, header.Len-8)
	re.Read(payload)
	if _, err := re.Read(payload); err != nil && err.Error() != "EOF" {
		return nil, err
	}

	p := &Packet{
		uid:       header.UID,
		funcID:    header.Func,
		seqNum:    seqNum,
		respExp:   respExp,
		errorCode: ErrorCode(errCode),
		callback:  callback,

		payload: payload,
	}

	return p, nil
}

// Decode decodes the payload of a packet into a number of variables
func (p *Packet) Decode(vars ...interface{}) error {

	re := bytes.NewReader(p.payload)

	for _, v := range vars {

		if err := binary.Read(re, binary.LittleEndian, v); err != nil {
			if err.Error() == "EOF" {
				return nil
			}

			return err
		}

	}

	return nil
}

// UID returns the UID of the packet source / destination
func (p *Packet) UID() uint32 {
	return p.uid
}

// Length returns the overall length (header + payload) of the packet
func (p *Packet) Length() uint8 {
	if p.payload == nil {
		return 8
	}

	return uint8(8 + len(p.payload))
}

// FunctionID returns the function ID of the packet
func (p *Packet) FunctionID() uint8 {
	return p.funcID
}

// SequenceNum returns the 4-bit sequence number of the packet.
func (p *Packet) SequenceNum() uint8 {
	return p.seqNum
}

// ResponseExpected returns wether the caller expects an answer
func (p *Packet) ResponseExpected() bool {
	return p.respExp
}

// ErrorID returns the ID of the error (or ECOkay)
func (p *Packet) ErrorID() ErrorCode {
	return p.errorCode
}

// Error returns the corresponding error for the error ID
func (p *Packet) Error() error {
	switch p.errorCode {
	case ECInvalidParam:
		return ErrInvalidParam

	case ECFuncNotSupported:
		return ErrFuncNotSupported
	}

	return nil
}

// Callback indicates if this packet is a callback
func (p *Packet) Callback() bool {
	return p.callback
}

// Payload returns the payload of the packet
func (p *Packet) Payload() []byte {
	return p.payload
}

// Serialize converts the packet into a byte slice for sending
func (p *Packet) Serialize(wr io.Writer, seqNum byte) error {
	// Send header and payload
	if err := p.writeHeader(wr, seqNum); err != nil {
		return err
	}

	return p.writePayload(wr)
}

func parseParams(params []interface{}) ([]byte, error) {

	wr := bytes.NewBuffer(make([]byte, 0))

	for _, p := range params {

		if err := binary.Write(wr, binary.LittleEndian, p); err != nil {
			return nil, err
		}

	}

	return wr.Bytes(), nil

}

func (p *Packet) writeHeader(wr io.Writer, seqNum byte) error {

	// Header structure
	header := &struct {
		UID    uint32
		Len    uint8
		FuncID uint8
		Seq    uint8
		Flags  uint8
	}{}

	// Fill header
	header.UID = p.uid
	header.Len = p.Length()
	header.FuncID = p.funcID
	seqNum = seqNum << 4
	if p.respExp {
		seqNum |= 0x08
	}
	header.Seq = uint8(seqNum)
	header.Flags = uint8(p.errorCode << 6)

	// Send header
	return binary.Write(wr, binary.LittleEndian, header)
}

func (p *Packet) writePayload(wr io.Writer) error {
	_, err := wr.Write(p.payload)
	return err
}

// Scans a byte stream for packets and returns them as byte arrays
func scanPacket(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// We are unable to read the length of the packet.
	if len(data) < 5 {
		if atEOF {
			return 0, nil, errors.New("EOF")
		}

		return 0, nil, nil
	}

	// Check the length of the data and the packet
	if len(data) >= int(data[4]) {
		fmt.Println(data[:data[4]])
		return int(data[4]), data[:data[4]], nil
	}

	// The packet is incomplete but we are at EOF
	if atEOF {
		return 0, nil, errors.New("EOF")
	}

	return 0, nil, nil
}

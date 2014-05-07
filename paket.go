package tinkerforge

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

type ErrorCode uint8

const (
	ECOkay             ErrorCode = 0
	ECInvalidParam               = 1
	ECFuncNotSupported           = 2
)

var (
	ErrInvalidParam     = errors.New("Invalid Parameter")
	ErrFuncNotSupported = errors.New("Function is not supported")
)

type Packet struct {
	uid       uint32
	funcId    uint8
	seqNum    uint8
	respExp   bool
	errorCode ErrorCode
	callback  bool

	payload []byte
}

// NewPacket creates a new packet to be sent to the TinkerForge daemon
func NewPacket(uid uint32, funcId uint8, respExp bool, params ...interface{}) Packet {

	return Packet{
		uid:       uid,
		funcId:    funcId,
		seqNum:    0,
		respExp:   respExp,
		errorCode: 0,
		callback:  false,
		payload:   parseParams(params),
	}

}

func ReadPacket(re io.Reader) Packet {

	header := struct {
		Uid   uint32
		Len   uint8
		Func  uint8
		Seq   uint8
		Flags uint8
	}{}

	if err := binary.Read(re, binary.LittleEndian, &header); err != nil {
		panic(err.Error())
	}

	respExp := header.Seq&0x08 != 0
	seqNum := header.Seq >> 4
	callback := seqNum == 0
	errCode := header.Flags >> 6

	payload := make([]byte, 0, header.Len-8)
	if err := binary.Read(re, binary.LittleEndian, &payload); err != nil {
		panic(err.Error())
	}

	return Packet{
		uid:       header.Uid,
		funcId:    header.Func,
		seqNum:    seqNum,
		respExp:   respExp,
		errorCode: ErrorCode(errCode),
		callback:  callback,

		payload: payload,
	}

}

func (p Packet) ReadPayload(vars ...interface{}) {

	re := bytes.NewBuffer(p.payload)

	if err := binary.Read(re, binary.LittleEndian, vars); err != nil {
		panic(err.Error())
	}

}

// UID returns the UID of the packet source / destination
func (p Packet) UID() uint32 {
	return p.uid
}

// Length returns the overall length (header + payload) of the packet
func (p Packet) Length() uint8 {
	if p.payload == nil {
		return 8
	}

	return uint8(8 + len(p.payload))
}

// FunctionID returns the function ID of the packet
func (p Packet) FunctionID() uint8 {
	return p.funcId
}

func (p Packet) SequenceNum() uint8 {
	return p.seqNum
}

// ResponseExpected returns wether the caller expects an answer
func (p Packet) ResponseExpected() bool {
	return p.respExp
}

// ErrorID returns the ID of the error (or ECOkay)
func (p Packet) ErrorID() ErrorCode {
	return p.errorCode
}

// Error returns the corresponding error for the error ID
func (p Packet) Error() error {
	switch p.errorCode {
	case ECInvalidParam:
		return ErrInvalidParam

	case ECFuncNotSupported:
		return ErrFuncNotSupported
	}

	return nil
}

func (p Packet) Callback() bool {
	return p.callback
}

// Serialize converts the packet into a byte slice for sending
func (p Packet) Serialize(wr io.Writer, seqNum byte) {

	p.writeHeader(wr, seqNum)
	p.writePayload(wr)

}

func parseParams(params []interface{}) []byte {

	wr := bytes.NewBuffer(make([]byte, 0))

	for _, p := range params {

		if err := binary.Write(wr, binary.LittleEndian, p); err != nil {
			panic(err.Error())
		}

	}

	return wr.Bytes()

}

func (p Packet) writeHeader(wr io.Writer, seqNum byte) {

	header := struct {
		Uid   uint32
		Len   uint8
		Func  uint8
		Seq   uint8
		Flags uint8
	}{}

	header.Uid = p.uid
	header.Len = p.Length()
	header.Func = p.funcId
	seqNum = seqNum << 4
	if p.respExp {
		seqNum |= 0x08
	}
	header.Seq = uint8(seqNum)
	header.Flags = uint8(p.errorCode << 6)

	if err := binary.Write(wr, binary.LittleEndian, header); err != nil {
		panic(err.Error())
	}

}

func (p Packet) writePayload(wr io.Writer) {

	if err := binary.Write(wr, binary.LittleEndian, p.payload); err != nil {
		panic(err.Error())
	}

}

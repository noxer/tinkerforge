package tinkerforge

import (
	"container/list"
	"net"
)

type tinkerforge struct {
	conn       net.Conn
	nextSeqNum chan byte

	adhoc      map[adhocKey]chan Packet
	callback   *list.List
	send       chan sendReq
	recv       chan Packet
	register   chan regCallback
	unregister chan regCallback

	exit chan struct{}
}

type adhocKey struct {
	uid    uint32
	funcId uint8
	seqNum uint8
}

type sendReq struct {
	packet   Packet
	response chan Packet
}

type regCallback struct {
	uid    uint32
	funcId uint8
	ch     chan Packet
}

// New establishes a new connection to a tinkerforge master brick
func New(host string) (*tinkerforge, error) {
	// Variables
	var err error
	t := &tinkerforge{
		adhoc:      make(map[adhocKey]chan Packet),
		callback:   list.New(),
		send:       make(chan sendReq),
		recv:       make(chan Packet),
		register:   make(chan regCallback),
		unregister: make(chan regCallback),
		exit:       make(chan struct{}),
	}

	// Establish connection
	t.conn, err = net.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	// Start the sequence number generator (and pregenerate 8 seqNumbers)
	t.nextSeqNum = make(chan byte, 8)
	go func() {

		seqNum := byte(1)
		for {
			select {
			case t.nextSeqNum <- seqNum:
			case <-t.exit:
				return
			}
			seqNum++
			if seqNum == 16 {
				seqNum = 1
			}
		}

	}()

	// Start the sender
	go func() {

		for {
			select {

			case p, ok := <-t.send:
				if !ok {
					return
				}

				seqNum := <-t.nextSeqNum
				p.packet.Serialize(t.conn, seqNum)
				if p.packet.ResponseExpected() && p.response != nil {
					t.addAdhoc(p.packet.UID(), p.packet.FunctionID(), seqNum, p.response)
				}

			case p, ok := <-t.recv:
				if !ok {
					return
				}

				if p.Callback() {
					t.forwardCallback(p)
				} else {
					t.forwardAdhoc(p)
				}

			case r, ok := <-t.register:
				if !ok {
					return
				}

				t.addCallback(r)

			case r, ok := <-t.unregister:
				if !ok {
					return
				}

				t.removeCallback(r)

			case <-t.exit:
				return

			}
		}

	}()

	// Start the reveiver
	go func() {
		defer recover()

		// Read until the connection dies
		for {
			t.recv <- readPacket(t.conn)
		}
	}()

	return t, nil
}

func (t *tinkerforge) addAdhoc(uid uint32, funcId uint8, seqNum uint8, ch chan Packet) {

	t.adhoc[adhocKey{uid, funcId, seqNum}] = ch

}

func (t *tinkerforge) addCallback(r regCallback) {

	t.callback.PushBack(r)

}

func (t *tinkerforge) removeCallback(r regCallback) {

	for e := t.callback.Front(); e != nil; e = e.Next() {

		if e.Value.(regCallback) == r {
			t.callback.Remove(e)
		}

	}

}

func (t *tinkerforge) forwardCallback(packet Packet) {

	for e := t.callback.Front(); e != nil; e = e.Next() {

		val := e.Value.(regCallback)
		if val.uid == packet.UID() && val.funcId == packet.FunctionID() {
			val.ch <- packet
		}

	}

}

func (t *tinkerforge) forwardAdhoc(packet Packet) {

	key := adhocKey{packet.UID(), packet.FunctionID(), packet.SequenceNum()}

	if ch, ok := t.adhoc[key]; ok {
		ch <- packet
		delete(t.adhoc, key)
	}

}

func (t *tinkerforge) Close() {
	t.conn.Close()
	close(t.exit)
	close(t.send)
}

func (t *tinkerforge) Send(packet Packet) {

	// When a response is expected: block for it
	if packet.Callback() {

		ch := make(chan Packet)
		t.send <- sendReq{packet, ch}
		<-ch

	} else {

		t.send <- sendReq{packet, nil}

	}

}

func (t *tinkerforge) Request(packet Packet, resp chan Packet) {

	t.send <- sendReq{packet, resp}

}

func (t *tinkerforge) RegisterCallback(uid uint32, funcId uint8, ch chan Packet) {

}

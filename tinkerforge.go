// Implementation of the tinkerforge protocol
// Author: Tim Scheuermann (https://github.com/noxer)
package tinkerforge

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Handler to get callbacks
type Handler interface {
	Handle(packet *Packet)
}

// respHandler for getting responses back
type respHandler struct {
	c chan *Packet
	t time.Duration
}

// Handles responses
func (r respHandler) Handle(p *Packet) {
	fmt.Println("Handle respHandler")

	// No timeout provided (don't!)
	if r.t == 0 {
		r.c <- p
		return
	}

	timer := time.NewTimer(r.t)

	select {
	case r.c <- p:
		timer.Stop()
	case <-timer.C:
		close(r.c)
	}
}

// Tinkerforge interface
type Tinkerforge interface {
	io.Closer
	Handler(uid uint32, funcId uint8, handler Handler)
	Send(packet *Packet) (*Packet, error)
}

// Tinkerforge structure
type tinkerforge struct {
	conn          io.ReadWriteCloser
	seqNum        chan byte
	handlers      map[handlerId]Handler
	handlersMutex sync.RWMutex

	sendQueue chan func()

	done chan struct{}
	wait sync.WaitGroup

	Timeout time.Duration
}

type handlerId struct {
	uid    uint32
	funcId uint8
	seqNum uint8
}

var (
	ErrTimeout = errors.New("Timeout while waiting for callback")
)

// New creates a new tinkerforge client
func New(host string) (Tinkerforge, error) {

	fmt.Println("New.")

	// Set standard host
	if host == "" {
		host = "localhost:4223"
	}

	// Resolve service address
	addr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		return nil, err
	}

	// Connect to service
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}

	// Build up structure
	tf := &tinkerforge{
		conn:      conn,
		seqNum:    make(chan byte, 8),
		handlers:  make(map[handlerId]Handler),
		sendQueue: make(chan func(), 8),
		done:      make(chan struct{}),
		Timeout:   10 * time.Second,
	}

	// Start the go routines
	go tf.seqNumGenerator() // Sequence number generator
	go tf.sender()          // Sender (queue for functions to send packets)
	go tf.receiver()        // Receiver

	// Okay
	return tf, nil
}

// Close closes the connection to the tinkerforge service
func (t *tinkerforge) Close() error {

	fmt.Println("Close.")

	// Close the channels
	close(t.done)
	close(t.sendQueue)

	// Close the tcp connection
	if err := t.conn.Close(); err != nil {
		return err
	}

	// Wait for termination
	t.wait.Wait()
	return nil
}

// Send sends a new packet to the service and returns the answer (if an answer is expected)
func (t *tinkerforge) Send(p *Packet) (*Packet, error) {

	fmt.Println("Send.")

	var packets chan *Packet

	errors := make(chan error, 1)

	// Create response channel in case we need it
	if p.ResponseExpected() {
		packets = make(chan *Packet, 1)
	}

	f := func() {

		fmt.Println("Send f.")

		// Generate sequence number
		seqNum := <-t.seqNum

		// Register callback for expected response (if any)
		if p.ResponseExpected() {
			t.handler(p.UID(), p.FunctionId(), seqNum, respHandler{c: packets, t: t.Timeout})
		}

		// Send packet
		if err := p.Serialize(t.conn, seqNum); err != nil {
			errors <- err
			return
		}

		// Close the error channel
		close(errors)
	}

	// Dispatch f
	t.sendQueue <- f

	// An error occurred
	if err := <-errors; err != nil {
		return nil, err
	}

	// Return depeneding of the expected response
	if p.ResponseExpected() {
		result, ok := <-packets
		if ok {
			return result, nil
		} else {
			// Timeout
			return nil, ErrTimeout
		}

	} else {
		return nil, nil
	}

}

// Handler registers a new handler for a packet
func (t *tinkerforge) Handler(uid uint32, funcId uint8, h Handler) {
	t.handler(uid, funcId, 0, h)
}

// handler registers any handler (internal)
func (t *tinkerforge) handler(uid uint32, funcId, seqNum uint8, h Handler) {

	fmt.Println("handler.")

	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	// Make the handler removable
	if h == nil {
		delete(t.handlers, handlerIdFromParam(uid, funcId, seqNum))
		return
	}

	// Add handler
	t.handlers[handlerIdFromParam(uid, funcId, seqNum)] = h
}

// Sequence number generator
func (t *tinkerforge) seqNumGenerator() {
	t.wait.Add(1)
	defer t.wait.Done()

	num := byte(1)
	for {
		select {
		case t.seqNum <- num:
		case <-t.done:
			return
		}
		num++
		if num == 16 {
			num = 1
		}
	}
}

// Sender executes the funktions in sendQueue
func (t *tinkerforge) sender() {
	t.wait.Add(1)
	defer t.wait.Done()

	// Execute all functions
	for f := range t.sendQueue {
		f()
	}
}

// Receiver listens on the TCP connection and exeecutes the handlers accordingly
func (t *tinkerforge) receiver() {
	t.wait.Add(1)
	defer t.wait.Done()

	// Set up scanner
	scanner := bufio.NewScanner(t.conn)
	scanner.Split(scanPacket)

	// Scan for packets
	for scanner.Scan() {
		// Parse the packet
		p, err := readPacket(scanner.Bytes())
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		// Call the handler
		t.handle(p)

		// Remove handler if it was not a callback
		if !p.Callback() {
			t.handler(p.UID(), p.FunctionId(), p.SequenceNum(), nil)
		}
	}
}

// handle searches for a matching hander for p and executes it
func (t *tinkerforge) handle(p *Packet) {
	t.handlersMutex.RLock()

	fmt.Println(handlerIdFromPacket(p))
	fmt.Println(t.handlers)

	if handler, ok := t.handlers[handlerIdFromPacket(p)]; ok {
		fmt.Println("Found handler exact")

		t.handlersMutex.RUnlock()
		handler.Handle(p)
	} else {
		// Maybe a wildcard?
		if handler, ok = t.handlers[handlerIdFromParam(0, p.FunctionId(), p.SequenceNum())]; ok {
			fmt.Println("Found handler wildcard")

			t.handlersMutex.RUnlock()
			handler.Handle(p)
		}
	}
}

// handlerIdFromParam creates a new handler ID from the params
func handlerIdFromParam(uid uint32, funcId, seqNum uint8) handlerId {
	return handlerId{
		uid:    uid,
		funcId: funcId,
		seqNum: seqNum,
	}
}

// handlerIdFromPacket creates a new handler ID from packet p
func handlerIdFromPacket(p *Packet) handlerId {
	return handlerId{
		uid:    p.UID(),
		funcId: p.FunctionId(),
		seqNum: p.SequenceNum(),
	}
}

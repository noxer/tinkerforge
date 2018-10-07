// Package tinkerforge implements the tinkerforge protocol
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
	Handler(uid uint32, funcID uint8, handler Handler)
	Send(packet *Packet) (*Packet, error)
}

// Tinkerforge structure
type tinkerforge struct {
	conn          io.ReadWriteCloser
	seqNum        chan byte
	handlers      map[handlerID]Handler
	handlersMutex sync.RWMutex

	sendQueue chan func()

	done chan struct{}
	wait sync.WaitGroup

	Timeout time.Duration
}

type handlerID struct {
	uid    uint32
	funcID uint8
	seqNum uint8
}

var (
	// ErrTimeout represents a timeout while waiting for a callback
	ErrTimeout = errors.New("Timeout while waiting for callback")
)

// New creates a new tinkerforge client
func New(host string) (Tinkerforge, error) {
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
		handlers:  make(map[handlerID]Handler),
		sendQueue: make(chan func(), 8),
		done:      make(chan struct{}),
		Timeout:   10 * time.Second,
	}

	// Start the go routines
	tf.wait.Add(3)
	go tf.seqNumGenerator() // Sequence number generator
	go tf.sender()          // Sender (queue for functions to send packets)
	go tf.receiver()        // Receiver

	// Okay
	return tf, nil
}

// Close closes the connection to the tinkerforge service
func (t *tinkerforge) Close() error {
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
	var packets chan *Packet

	errors := make(chan error, 1)

	// Create response channel in case we need it
	if p.ResponseExpected() {
		packets = make(chan *Packet, 1)
	}

	f := func() {
		// Generate sequence number
		seqNum := <-t.seqNum

		// Register callback for expected response (if any)
		if p.ResponseExpected() {
			t.handler(p.UID(), p.FunctionID(), seqNum, respHandler{c: packets, t: t.Timeout})
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

	// Return depending of the expected response
	if p.ResponseExpected() {
		result, ok := <-packets
		if ok {
			return result, nil
		}
		// Timeout
		return nil, ErrTimeout
	}

	return nil, nil
}

// Handler registers a new handler for a packet
func (t *tinkerforge) Handler(uid uint32, funcID uint8, h Handler) {
	t.handler(uid, funcID, 0, h)
}

// handler registers any handler (internal)
func (t *tinkerforge) handler(uid uint32, funcID, seqNum uint8, h Handler) {
	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	// Make the handler removable
	if h == nil {
		delete(t.handlers, handlerIDFromParam(uid, funcID, seqNum))
		return
	}

	// Add handler
	t.handlers[handlerIDFromParam(uid, funcID, seqNum)] = h
}

// Sequence number generator
func (t *tinkerforge) seqNumGenerator() {
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
	defer t.wait.Done()

	// Execute all functions
	for f := range t.sendQueue {
		f()
	}
}

// Receiver listens on the TCP connection and exeecutes the handlers accordingly
func (t *tinkerforge) receiver() {
	defer t.wait.Done()

	// Set up scanner
	scanner := bufio.NewScanner(t.conn)
	scanner.Split(scanPacket)

	// Scan for packets
	for scanner.Scan() {
		// Parse the packet
		p, err := readPacket(scanner.Bytes())
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Call the handler
		t.handle(p)

		// Remove handler if it was not a callback
		if !p.Callback() {
			t.handler(p.UID(), p.FunctionID(), p.SequenceNum(), nil)
		}
	}
}

// handle searches for a matching hander for p and executes it
func (t *tinkerforge) handle(p *Packet) {
	t.handlersMutex.RLock()

	var handler Handler
	handler, ok := t.handlers[handlerIDFromPacket(p)]
	if !ok {
		// Maybe a wildcard?
		handler = t.handlers[handlerIDFromParam(0, p.FunctionID(), p.SequenceNum())]
	}

	t.handlersMutex.RUnlock()
	if handler != nil {
		handler.Handle(p)
	}
}

// handlerIdFromParam creates a new handler ID from the params
func handlerIDFromParam(uid uint32, funcID, seqNum uint8) handlerID {
	return handlerID{
		uid:    uid,
		funcID: funcID,
		seqNum: seqNum,
	}
}

// handlerIdFromPacket creates a new handler ID from packet p
func handlerIDFromPacket(p *Packet) handlerID {
	return handlerID{
		uid:    p.UID(),
		funcID: p.FunctionID(),
		seqNum: p.SequenceNum(),
	}
}

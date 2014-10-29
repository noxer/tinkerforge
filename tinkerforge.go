// tinkerforge.go
// Implementation of the tinkerforge protocol
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
package tinkerforge

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sync"
)

// Handler to get callbacks
type Handler interface {
	Handle(packet *Packet)
}

// respHandler for getting responses back
type respHandler chan *Packet

// Handles responses
func (r respHandler) Handle(p *Packet) {
	fmt.Println("Handle respHandler")

	r <- p
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
}

type handlerId struct {
	uid    uint32
	funcId uint8
	seqNum uint8
}

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
			t.handler(p.UID(), p.FunctionId(), seqNum, respHandler(packets))
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
		return <-packets, nil
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

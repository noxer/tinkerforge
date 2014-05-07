tinkerforge
===========

A go implementation of the tinkerforge protocol.

Example
-------

´´´go
package main

import (
    "github.com/Noxer/tinkerforge"
)
  
func main() {
    // Open a new connection to brickd (we ignore the error)
    t, _ := tinkerforge.New("localhost:4280")

    // Calls function 32 of bricklet 3631747890
    // no response expected
    // arguments are -239, 69 and -223 as int16
    t.Send(tinkerforge.NewPacket(3631747890, 32, false, int16(-239), int16(60), int16(-223)))
    
    // Calls the same function but expects an answer
    resp := make(chan Packet, 1) // Buffered so we don't block
    t.Request(tinkerforge.NewPacket(3631747890, 32, true, int16(-239), int16(60), int16(-223)), resp)
    packet := <-resp

    // do something with the response packet

    t.Close()
}
´´´
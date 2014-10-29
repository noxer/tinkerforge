tinkerforge
===========

[![GoDoc](https://godoc.org/github.com/noxer/tinkerforge?status.svg)](https://godoc.org/github.com/noxer/tinkerforge)

A go implementation of the tinkerforge protocol.

Install
-------
```
go get github.com/noxer/tinkerforge
```

Example
-------

The tinkerforge package can be used directly:
```go
package main

import (
    "github.com/noxer/tinkerforge"
)
  
func main() {
    // Open a new connection to brickd (we ignore the error)
    t, _ := tinkerforge.New("localhost:4223")
    defer t.Close()

    // Call function 1 of brick(let) 123456, don't expect an answer and send three integers (again we ignore the error)
    p, _ := tinkerforge.NewPacket(123456, 1, false, int32(42), int32(43), int32(44))

    // Send the packet
    t.Send(p)
}
```

There are high level functions in the "ledstrip"package:
```go
package main

import (
    "github.com/noxer/tinkerforge"
    "github.com/noxer/tinkerforge/ledstrip"
)
  
func main() {
    // Open a new connection to brickd (we ignore the error)
    t, _ := tinkerforge.New("localhost:4223")
    defer t.Close()

    // Wrap with the ledstrip controller and set the bricklet ID to 123456 (error ignored)
    l, _ := ledstrip.New(t, 123456)

    // Set the color of the first LED to red (if it appears in blue you need to set the color mapping)
    l.SetRGBValues(0, []ledstrip.Color{ledstrip.Color{255, 0, 0}})
}
```

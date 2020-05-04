// SPDX-License-Identifier: GPL-3.0-only

package nscon

import (
	"encoding/hex"
	"log"
	"os"
	"time"
)

var SPI_ROM_DATA = map[byte][]byte{
	0x60: []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0x03, 0xa0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02, 0xff, 0xff, 0xff, 0xff,
		0xf0, 0xff, 0x89, 0x00, 0xf0, 0x01, 0x00, 0x40, 0x00, 0x40, 0x00, 0x40, 0xf9, 0xff, 0x06, 0x00,
		0x09, 0x00, 0xe7, 0x3b, 0xe7, 0x3b, 0xe7, 0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xba, 0x15, 0x62,
		0x11, 0xb8, 0x7f, 0x29, 0x06, 0x5b, 0xff, 0xe7, 0x7e, 0x0e, 0x36, 0x56, 0x9e, 0x85, 0x60, 0xff,
		0x32, 0x32, 0x32, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x50, 0xfd, 0x00, 0x00, 0xc6, 0x0f, 0x0f, 0x30, 0x61, 0x96, 0x30, 0xf3, 0xd4, 0x14, 0x54, 0x41,
		0x15, 0x54, 0xc7, 0x79, 0x9c, 0x33, 0x36, 0x63, 0x0f, 0x30, 0x61, 0x96, 0x30, 0xf3, 0xd4, 0x14,
		0x54, 0x41, 0x15, 0x54, 0xc7, 0x79, 0x9c, 0x33, 0x36, 0x63, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	},
	0x80: []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xb2, 0xa1, 0xbe, 0xff, 0x3e, 0x00, 0xf0, 0x01, 0x00, 0x40,
		0x00, 0x40, 0x00, 0x40, 0xfe, 0xff, 0xfe, 0xff, 0x08, 0x00, 0xe7, 0x3b, 0xe7, 0x3b, 0xe7, 0x3b,
	},
}

type ButtonMap struct {
	Dpad struct {
		Up, Down, Left, Right uint8
	}
	Button struct {
		A, B, X, Y, R, ZR, L, ZL   uint8
		Home, Plus, Minus, Capture uint8
	}
	Stick struct {
		Left, Right struct {
			X, Y float64
		}
	}
}

type Controller struct {
	fp          *os.File
	count       uint8
	stopCounter chan struct{}
	stopInput   chan struct{}
	Button      ButtonMap
}

func NewController(fp *os.File) *Controller {
	return &Controller{
		fp:          fp,
		stopCounter: make(chan struct{}),
		stopInput:   make(chan struct{}),
	}
}

// Close closes all channel and device file
func (c *Controller) Close() {
	close(c.stopCounter)
	close(c.stopInput)
	c.fp.Close()
}

func (c *Controller) startCounter() {
	ticker := time.NewTicker(time.Millisecond * 5)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.count++
			case <-c.stopCounter:
				return
			}
		}
	}()
}

func (c *Controller) startInputReport() {
	ticker := time.NewTicker(time.Millisecond * 30)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				dpad := c.Button.Dpad.Left<<3 |
					c.Button.Dpad.Right<<2 |
					c.Button.Dpad.Up<<1 |
					c.Button.Dpad.Down
				c.write([]byte{0x30, c.count, 0x81, 0x00, 0x80, dpad, 0x00, 0x08,
					0x80, 0x00, 0x08, 0x80, 0x00})
			case <-c.stopInput:
				return
			}
		}
	}()
}

func (c *Controller) uart(ack byte, subCmd byte, data []byte) {
	c.write(append([]byte{0x21, c.count, 0x81, 0x00, 0x80, 0x00, 0x00, 0x08,
		0x80, 0x00, 0x08, 0x80, 0x00, ack, subCmd}, data...))
}

func (c *Controller) write(buf []byte) {
	data := append(buf, make([]byte, 64-len(buf))...)
	c.fp.Write(data)
	if buf[0] != 0x30 {
		log.Println("write:", hex.EncodeToString(data))
	}
}

// Connect begins connection to device
func (c *Controller) Connect() {
	c.startCounter()
	go func() {
		buf := make([]byte, 128)

		for {
			n, err := c.fp.Read(buf)
			log.Println("read:", hex.EncodeToString(buf[:n]), err)
			switch buf[0] {
			case 0x80:
				switch buf[1] {
				case 0x01:
					c.write([]byte{0x81, buf[1], 0x00, 0x03, 0x00, 0x00, 0x5e, 0x00, 0x53, 0x5e})
				case 0x02, 0x03:
					c.write([]byte{0x81, buf[1]})
				case 0x04:
					c.startInputReport()
				case 0x05:
					close(c.stopInput)
					c.stopInput = make(chan struct{})
				}
			case 0x01:
				switch buf[10] {
				case 0x01: // Bluetooth manual pairing
					c.uart(0x81, buf[10], []byte{0x03, 0x01})
				case 0x02: // Request device info
					c.uart(0x82, buf[10], []byte{0x03, 0x48, 0x03,
						0x02, 0x5e, 0x53, 0x00, 0x5e, 0x00, 0x00, 0x03, 0x01})
				case 0x03, 0x08, 0x30, 0x38, 0x40, 0x41, 0x48: // Empty response
					c.uart(0x80, buf[10], []byte{})
				case 0x04: // Empty response
					c.uart(0x80, buf[10], []byte{})
				case 0x10: // Read SPI ROM
					data, ok := SPI_ROM_DATA[buf[12]]
					if ok {
						c.uart(0x90, buf[10], append(buf[11:16],
							data[buf[11]:buf[11]+buf[15]]...))
						log.Printf("Read SPI address: %02x%02x[%d] %v\n", buf[12], buf[11], buf[15], data[buf[11]:buf[11]+buf[15]])
					} else {
						log.Printf("Unknown SPI address: %02x[%d]\n", buf[12], buf[15])
					}
				case 0x21:
					c.uart(0xa0, buf[10], []byte{0x01, 0x00, 0xff, 0x00, 0x03, 0x00, 0x05, 0x01})
				default:
					log.Println("UART unknown request", buf[10], buf)
				}

			case 0x00:
			case 0x10:
			default:
				log.Println("unknown request", buf[0])
			}
		}
	}()
}

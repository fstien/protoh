// main.go
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Println("listening on 8080")

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("error accepting: ", err)
			continue
		}

		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	prices := make(map[int32]int32)
	buf := make([]byte, 9)

	for {
		n, err := io.ReadFull(conn, buf)
		if err != nil || n != 9 {
			fmt.Printf("read error: %s, n: %d", err.Error(), n)
			return
		}

		i1 := int32(binary.BigEndian.Uint32(buf[1:5]))
		i2 := int32(binary.BigEndian.Uint32(buf[5:9]))

		switch buf[0] {
		case 'I':
			ts := i1
			price := i2

			prices[ts] = price
		case 'Q':
			mintime := i1
			maxtime := i2

			sum := 0
			c := 0

			for ts, p := range prices {
				if ts >= mintime && ts <= maxtime {
					sum += int(p)
					c++
				}
			}

			mean := 0
			if c > 0 {
				mean = sum / c
			}

			bytes := make([]byte, 4)
			binary.BigEndian.PutUint32(bytes, uint32(mean))

			n, err := conn.Write(bytes)
			if err != nil || n != 4 {
				fmt.Printf("failed to respond with mean %s, %d", err.Error(), n)
				return
			}
		default:
			fmt.Println("invalid request: ", buf[0])
			return
		}
	}
}

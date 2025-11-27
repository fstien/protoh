// main.go
package main

import (
	"errors"
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
	buf := make([]byte, 1024)
	defer conn.Close()

	for {
		nR, errR := conn.Read(buf)
		nW, errW := conn.Write(buf[:nR])

		if errR != nil {
			if errors.Is(errR, io.EOF) {
				fmt.Println("read EOF")
				return
			}
			fmt.Println("read error: ", errR)
			return
		}

		if errW != nil {
			fmt.Println("write error: ", errW)
			return
		}

		if nR != nW {
			fmt.Printf("nR: %d, nW: %d", nR, nW)
			return
		}
	}
}

package main

import (
	"log"
	"net"
)

func main() {
	addr, err := net.ResolveUDPAddr("udp", ":8080")
	if err != nil {
		log.Fatalf("resolve error: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	defer conn.Close()

	log.Println("UDP echo server listening on :8080")

	buf := make([]byte, 1000)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		log.Printf("received %d bytes from %s", n, clientAddr)

		_, err = conn.WriteToUDP(buf[:n], clientAddr)
		if err != nil {
			log.Printf("write error: %v", err)
		}
	}
}

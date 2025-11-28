package main

import (
	"fmt"
	"log"
	"net"
	"strings"
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

	kv := make(map[string]string)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		log.Printf("received %d bytes from %s", n, clientAddr)

		req := string(buf[:n])

		if string(req) == "version" {
			_, err = conn.WriteToUDP([]byte("version=Francois' kv store"), clientAddr)
			if err != nil {
				log.Printf("write error: %v", err)
			}
			continue
		}

		if strings.Contains(req, "=") {
			k, v, _ := strings.Cut(req, "=")

			if k == "version" {
				continue
			}

			kv[k] = v
		} else {
			rsp := kv[req]

			_, err = conn.WriteToUDP([]byte(fmt.Sprintf("%s=%s", req, rsp)), clientAddr)
			if err != nil {
				log.Printf("write error: %v", err)
			}
		}
	}
}

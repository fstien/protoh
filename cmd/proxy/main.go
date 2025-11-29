package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Println("listening on 8080")

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		ln.Close()
		cancel()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		go handleConn(ctx, conn)
	}
}

func handleConn(ctx context.Context, client net.Conn) {
	defer client.Close()

	clientReader := bufio.NewReader(client)

	server, err := net.Dial("tcp", "chat.protohackers.com:16963")
	if err != nil {
		fmt.Println("failed to dial", err)
		return
	}
	defer server.Close()

	serverReader := bufio.NewReader(server)

	ctx, cancel := context.WithCancel(ctx)

	go forward(cancel, clientReader, server, "client")
	go forward(cancel, serverReader, client, "server")

	<-ctx.Done()
}

func forward(cancel context.CancelFunc, src *bufio.Reader, dst net.Conn, label string) {
	for {
		msg, err := src.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("failed to read from %s: %v", label, err)
			}
			cancel()
			return
		}
		_, err = dst.Write([]byte(RewriteBoguscoin(msg)))
		if err != nil {
			log.Printf("failed to write to %s: %v", label, err)
			cancel()
			return
		}
	}
}

const TonysAddress = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

var boguscoinRegex = regexp.MustCompile(`^7[a-zA-Z0-9]{25,34}$`)

func RewriteBoguscoin(message string) string {
	suffix := ""
	if len(message) > 0 && message[len(message)-1] == '\n' {
		suffix = "\n"
		message = message[:len(message)-1]
	}

	parts := strings.Split(message, " ")
	for i, part := range parts {
		if boguscoinRegex.MatchString(part) {
			parts[i] = TonysAddress
		}
	}
	return strings.Join(parts, " ") + suffix
}

package main

import (
	"bufio"
	"context"
	"fmt"
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
			continue
		}

		go handleConn(ctx, conn)
	}
}

func handleConn(ctx context.Context, upstream net.Conn) {
	defer upstream.Close()

	upReader := bufio.NewReader(upstream)

	down, err := net.Dial("tcp", "chat.protohackers.com:16963")
	if err != nil {
		fmt.Println("failed to dial", err)
		return
	}

	downReader := bufio.NewReader(down)

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		for {
			msg, err := downReader.ReadString('\n')
			if err != nil {
				log.Println("failed to read from downstream", err)
				cancel()
				return
			}

			_, err = upstream.Write([]byte(RewriteBoguscoin(msg)))
			if err != nil {
				log.Println("failed to write to upstream", err)
				cancel()
				return
			}
		}
	}()

	go func() {
		for {
			msg, err := upReader.ReadString('\n')
			if err != nil {
				log.Println("failed to read from upstrean", err)
				cancel()
				return
			}

			_, err = down.Write([]byte(RewriteBoguscoin(msg)))
			if err != nil {
				log.Println("failed to write to downstream", err)
				cancel()
				return
			}
		}
	}()

	<-ctx.Done()
}

const TonysAddress = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

var boguscoinRegex = regexp.MustCompile(`^7[a-zA-Z0-9]{25,34}$`)

// RewriteBoguscoin replaces all Boguscoin addresses in a message with Tony's address
func RewriteBoguscoin(message string) string {
	// Strip trailing newline for processing, we'll add it back
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

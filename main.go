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
	"syscall"
)

var re = regexp.MustCompile(`(^|\s)(7[a-zA-Z0-9]{25,34})(?=\s|$)`)

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

			_, err = upstream.Write([]byte(rewrite(msg)))
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

			_, err = down.Write([]byte(rewrite(msg)))
			if err != nil {
				log.Println("failed to write to downstream", err)
				cancel()
				return
			}
		}
	}()

	<-ctx.Done()
}

const tonyAddress = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

// regex explanation:
// (?m)           -> multi-line mode so ^ and $ match start/end of each line
// (^| )          -> capture either start-of-line or a single space before the address
// 7[0-9A-Za-z]{25,34} -> the address: starts with 7 plus 25..34 alnum (total length 26..35)
// ($| )          -> capture either end-of-line or a single space after the address
var bogusRe = regexp.MustCompile(`(?m)(^| )7[0-9A-Za-z]{25,34}($| )`)

// RewriteBoguscoin rewrites all Boguscoin addresses in the input string to Tony's address.
// It preserves any leading/trailing space captured around the address.
func RewriteBoguscoin(s string) string {
	// replacement uses $1 and $2 to preserve the captured surrounding tokens
	repl := "$1" + tonyAddress + "$2"
	return bogusRe.ReplaceAllString(s, repl)
}

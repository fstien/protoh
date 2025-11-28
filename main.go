// main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
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

	b := newMessageBroker(ctx)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("error accepting: ", err)
			continue
		}

		go handleConn(ctx, b, conn)
	}
}

type commandType string

const (
	subscribe   commandType = "subscribe"
	send        commandType = "send"
	unsubscribe commandType = "unsubscribe"
)

type command struct {
	t        commandType
	name     string
	ch       chan message
	content  string
	memberCh chan []string
}

type messageBroker struct {
	commandCh chan command
}

type message struct {
	sender  string
	content string
}

func newMessageBroker(ctx context.Context) *messageBroker {
	b := &messageBroker{
		commandCh: make(chan command),
	}
	go b.loop(ctx)
	return b
}

func (b *messageBroker) loop(ctx context.Context) {
	sub := make(map[string]chan message)

	for {
		select {
		case <-ctx.Done():
			return
		case c := <-b.commandCh:
			switch c.t {
			case subscribe:
				members := make([]string, 0)
				for m, s := range sub {
					members = append(members, m)
					s <- message{content: fmt.Sprintf("%s has joined the room", c.name)}
				}
				sub[c.name] = c.ch
				c.memberCh <- members

			case send:
				for name, s := range sub {
					if name != c.name {
						s <- message{
							sender:  c.name,
							content: c.content,
						}
					}
				}

			case unsubscribe:
				for n, s := range sub {
					if n != c.name {
						s <- message{content: fmt.Sprintf("%s has left the room", c.name)}
					}
				}
				delete(sub, c.name)
			}
		}
	}
}

func (b *messageBroker) send(msg message) {
	b.commandCh <- command{
		t:       send,
		name:    msg.sender,
		content: msg.content,
	}
}

func (b *messageBroker) sub(name string) ([]string, chan message, error) {
	c := make(chan message, 10)
	memberCh := make(chan []string)

	b.commandCh <- command{
		t:        subscribe,
		name:     name,
		ch:       c,
		memberCh: memberCh,
	}

	members := <-memberCh
	return members, c, nil
}

func (b *messageBroker) unsub(name string) {
	b.commandCh <- command{
		t:    unsubscribe,
		name: name,
	}
}

func handleConn(ctx context.Context, b *messageBroker, conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	_, err := conn.Write([]byte("Welcome to budgetchat! What shall I call you?\n"))
	if err != nil {
		fmt.Println("failed to write greeting", err)
		return
	}

	name, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("read error: %s", err.Error())
		return
	}

	name = strings.TrimSuffix(name, "\n")

	if len(name) < 1 || len(name) > 1000 || !IsASCIIAlphanumeric(name) {
		conn.Write([]byte("invalid name"))
		return
	}

	members, c, err := b.sub(name)
	if err != nil {
		fmt.Println("failed to subscribe", err)
		return
	}

	_, err = conn.Write([]byte(fmt.Sprintf("* The room contains: %s\n", strings.Join(members, ", "))))
	if err != nil {
		fmt.Println("failed to write members", err)
	}

	defer b.unsub(name)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func(cancel context.CancelFunc, b *messageBroker, r *bufio.Reader) {
		for {
			msg, err := reader.ReadString('\n')
			msg = strings.TrimSuffix(msg, "\n")
			if err != nil {
				cancel()
				return
			}
			b.send(message{
				sender:  name,
				content: msg,
			})
		}
	}(cancel, b, reader)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c:
			if msg.sender == "" {
				_, err = conn.Write([]byte(fmt.Sprintf("* %s\n", msg.content)))
				if err != nil {
					fmt.Println("failed to write msg", err)
					return
				}
			} else {
				_, err = conn.Write([]byte(fmt.Sprintf("[%s] %s\n", msg.sender, msg.content)))
				if err != nil {
					fmt.Println("failed to write msg", err)
					return
				}
			}
		}
	}

}

func IsASCIIAlphanumeric(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

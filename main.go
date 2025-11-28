// main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	fmt.Println("listening on 8080")

	b := newMessageBroker()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("error accepting: ", err)
			continue
		}

		go handleConn(b, conn)
	}
}

type commandType string

const (
	subscribe   commandType = "subscribe"
	send        commandType = "send"
	unsubscribe commandType = "unsubscribe"
	listMembers commandType = "list_members"
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

func newMessageBroker() *messageBroker {
	b := &messageBroker{
		commandCh: make(chan command),
	}
	go b.loop()
	return b
}

func (b *messageBroker) loop() {
	sub := make(map[string]chan message)

	for {
		select {
		case c := <-b.commandCh:
			switch c.t {
			case subscribe:
				for _, s := range sub {
					s <- message{content: fmt.Sprintf("%s has joined the room", c.name)}
				}
				sub[c.name] = c.ch

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

			case listMembers:
				m := make([]string, 0)

				for n, _ := range sub {
					m = append(m, n)
				}

				c.memberCh <- m
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

func (b *messageBroker) sub(name string) (chan message, error) {
	c := make(chan message, 10)

	b.commandCh <- command{
		t:    subscribe,
		name: name,
		ch:   c,
	}

	return c, nil
}

func (b *messageBroker) members() []string {
	c := make(chan []string)

	b.commandCh <- command{
		t:        listMembers,
		memberCh: c,
	}

	m := <-c
	return m
}

func (b *messageBroker) unsub(name string) {
	b.commandCh <- command{
		t:    unsubscribe,
		name: name,
	}
}

func handleConn(b *messageBroker, conn net.Conn) {
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

	_, err = conn.Write([]byte(fmt.Sprintf("* The room contains: %s\n", strings.Join(b.members(), ", "))))
	if err != nil {
		fmt.Println("failed to write members", err)
	}

	c, err := b.sub(name)
	if err != nil {
		fmt.Println("failed to subscribe", err)
		return
	}
	defer b.unsub(name)

	ctx, cancel := context.WithCancel(context.Background())

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
				}
			} else {
				_, err = conn.Write([]byte(fmt.Sprintf("[%s] %s\n", msg.sender, msg.content)))
				if err != nil {
					fmt.Println("failed to write msg", err)
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

// main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
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

type messageBroker struct {
	subscribers map[string]chan message
	mu          sync.Mutex
}

func newMessageBroker() *messageBroker {
	return &messageBroker{
		subscribers: make(map[string]chan message),
		mu:          sync.Mutex{},
	}
}

func (b *messageBroker) send(msg message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for name, c := range b.subscribers {
		if name != msg.sender {
			c <- msg
		}
	}
}

func (b *messageBroker) sub(name string) (chan message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscribers[name] != nil {
		return nil, fmt.Errorf("subscriber already exists with name %s", name)
	}

	for _, s := range b.subscribers {
		s <- message{content: fmt.Sprintf("%s has joined the room", name)}
	}

	c := make(chan message, 10)
	b.subscribers[name] = c
	return c, nil
}

func (b *messageBroker) members() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	m := []string{}

	for s, _ := range b.subscribers {
		m = append(m, s)
	}

	return m
}

func (b *messageBroker) unsub(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, s := range b.subscribers {
		s <- message{content: fmt.Sprintf("%s has left the room", name)}
	}

	delete(b.subscribers, name)
}

type message struct {
	sender  string
	content string
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

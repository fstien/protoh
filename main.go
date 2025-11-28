// main.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
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

type Req struct {
	Method *string `json:"method"`
	Number *any    `json:"number"`
}

type Rsp struct {
	Method string `json:"method"`
	Prime  bool   `json:"prime"`
}

type Malformed struct {
	Error string `json:"error"`
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)

	for {
		reqStr, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("read error: ", err)
			return
		}

		req := Req{}
		err = json.Unmarshal([]byte(reqStr), &req)
		if err != nil {
			fmt.Println("unmarshal error: ", err)
			return
		}

		if req.Number == nil {
			j, _ := json.Marshal(&Malformed{Error: "missing_number"})
			conn.Write(append(j, byte('\n')))
			return
		}
		if req.Method == nil {
			j, _ := json.Marshal(&Malformed{Error: "missing_method"})
			conn.Write(append(j, byte('\n')))
			return
		}

		if *req.Method != "isPrime" {
			j, _ := json.Marshal(&Malformed{Error: "invalid_method"})
			conn.Write(append(j, byte('\n')))
			return
		}

		n := 0

		switch (*req.Number).(type) {
		case string:
			j, _ := json.Marshal(&Malformed{Error: "invalid_number"})
			conn.Write(append(j, byte('\n')))
			return
		case float64:
			n = int((*req.Number).(float64))
		case int:
			n = (*req.Number).(int)
		}

		rsp := &Rsp{
			Method: "isPrime",
			Prime:  IsPrime(n),
		}
		j, err := json.Marshal(rsp)
		if err != nil {
			fmt.Println("failed to marshal", err)
			return
		}

		_, err = conn.Write(append(j, byte('\n')))
		if err != nil {
			fmt.Println("failed to write", err)
			return
		}
	}
}

func IsPrime(n int) bool {
	if n < 2 {
		return false
	}
	if n == 2 || n == 3 {
		return true
	}
	if n%2 == 0 {
		return false
	}

	// Only check odd numbers up to √n
	limit := int(math.Sqrt(float64(n)))
	for i := 3; i <= limit; i += 2 {
		if n%i == 0 {
			return false
		}
	}
	return true
}

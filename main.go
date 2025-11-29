package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	t := newTicketDispatcher(ctx)

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

		go handleConn(ctx, t, conn)
	}
}

const (
	commandPlate            = "plate"
	commandRegisterDispatch = "register_dispatch"
)

type ticket struct {
	plate string
	road  uint16
	mile1 uint16
	ts1   uint32
	mile2 uint16
	ts2   uint32
	speed uint16
}

type command struct {
	t string

	// plate
	road  uint16
	mile  uint16
	limit uint16
	plate string
	ts    uint32

	// registerDispatch
	ticketCh chan ticket
	roads    []uint16
}

type ticketDispatcher struct {
	commandCh chan command
}

func newTicketDispatcher(ctx context.Context) *ticketDispatcher {
	t := &ticketDispatcher{
		commandCh: make(chan command, 100),
	}
	go t.loop(ctx)
	return t
}

func (t *ticketDispatcher) loop(ctx context.Context) {
	dispatcherByRoad := make(map[uint16][]chan ticket)
	carTsByMileByRoadByPlate := make(map[string]map[uint16]map[uint16]uint32)
	ticketsPerDayByPlate := make(map[string]map[int]bool)
	pendingTicketsByRoad := make(map[uint16][]ticket)

	for {
		select {
		case <-ctx.Done():
			return
		case cd := <-t.commandCh:
			switch cd.t {
			case commandPlate:
				fmt.Printf("plate road: %d, mile %d, limit %d, %s, ts: %d \n", cd.road, cd.mile, cd.limit, cd.plate, cd.ts)

				if carTsByMileByRoadByPlate[cd.plate] != nil {
					if carTsByMileByRoadByPlate[cd.plate][cd.road] != nil {
						for m, ts := range carTsByMileByRoadByPlate[cd.plate][cd.road] {
							fmt.Printf("m: %d, ts: %d (plate: %s, road %d) \n", m, ts, cd.plate, cd.road)

							var dist uint16
							if cd.mile > m {
								dist = cd.mile - m
							} else {
								dist = m - cd.mile
							}

							var duration uint32
							if cd.ts > ts {
								duration = cd.ts - ts
							} else {
								duration = ts - cd.ts
							}

							// convert seconds to hours
							durationF := float64(duration) / float64(60*60)
							fmt.Printf("dist %d, duration %d, durationF %f\n", dist, duration, durationF)

							speed := float64(dist) / durationF

							fmt.Printf("speed %f, limit %d\n", speed, cd.limit)

							if speed > float64(cd.limit) {
								day := int(math.Floor(float64(cd.ts) / float64(86400)))
								if ticketsPerDayByPlate[cd.plate] != nil {
									if ticketsPerDayByPlate[cd.plate][day] == true {
										// ticket already issued
										continue
									}
								}

								ti := ticket{
									plate: cd.plate,
									road:  cd.road,
									mile1: m,
									ts1:   ts,
									mile2: cd.mile,
									ts2:   cd.ts,
									speed: uint16(math.Round(speed)),
								}

								dispatcher, ok := dispatcherByRoad[cd.road]
								if !ok {
									pendingTicketsByRoad[cd.road] = append(pendingTicketsByRoad[cd.road], ti)
								} else {
									fmt.Printf("dipatching %+v \n", ti)
									dispatcher[rand.Intn(len(dispatcher))] <- ti
								}

								if ticketsPerDayByPlate[cd.plate] == nil {
									ticketsPerDayByPlate[cd.plate] = make(map[int]bool)
								}
								ticketsPerDayByPlate[cd.plate][day] = true
							}

						}
					}
				}

				if carTsByMileByRoadByPlate[cd.plate] == nil {
					carTsByMileByRoadByPlate[cd.plate] = make(map[uint16]map[uint16]uint32)
					carTsByMileByRoadByPlate[cd.plate][cd.road] = make(map[uint16]uint32)
				}

				carTsByMileByRoadByPlate[cd.plate][cd.road][cd.mile] = cd.ts

			case commandRegisterDispatch:
				for _, r := range cd.roads {
					dispatcherByRoad[r] = append(dispatcherByRoad[r], cd.ticketCh)

					for _, ti := range pendingTicketsByRoad[r] {
						cd.ticketCh <- ti
					}
				}
			default:
				fmt.Println("unknown command: ", cd.t)
			}
		}
	}
}

func (t *ticketDispatcher) plate(road, mile, limit uint16, plate string, ts uint32) {
	t.commandCh <- command{
		t:     commandPlate,
		road:  road,
		mile:  mile,
		limit: limit,
		plate: plate,
		ts:    ts,
	}
}

func (t *ticketDispatcher) registerDispatcher(roads []uint16) chan ticket {
	ticketCh := make(chan ticket, 10)
	t.commandCh <- command{
		t:        commandRegisterDispatch,
		ticketCh: ticketCh,
		roads:    roads,
	}
	return ticketCh
}

const (
	IAmCamera     = 0x80
	plate         = 0x20
	WantHeartbeat = 0x40
	IAmDispatcher = 0x81
	Ticket        = 0x21
)

func handleConn(ctx context.Context, t *ticketDispatcher, client net.Conn) {
	defer client.Close()

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	for {
		msgType := make([]byte, 1)
		_, err := io.ReadFull(client, msgType)
		if err != nil {
			if err != io.EOF {
				fmt.Println("failed to read message type", err)
			}
			return
		}

		switch msgType[0] {
		case IAmCamera:
			body := make([]byte, 6)
			_, err := io.ReadFull(client, body)
			if err != nil {
				if err != io.EOF {
					fmt.Println("failed to read IAmCamera body", err)
				}
				return
			}

			road := binary.BigEndian.Uint16(body[0:2])
			mile := binary.BigEndian.Uint16(body[2:4])
			limit := binary.BigEndian.Uint16(body[4:6])

			for {
				msgType = make([]byte, 1)
				_, err := io.ReadFull(client, msgType)
				if err != nil {
					if err != io.EOF {
						fmt.Println("failed to read message type", err)
					}
					return
				}

				switch msgType[0] {
				case plate:
					plateSize := make([]byte, 1)
					_, err := io.ReadFull(client, plateSize)
					if err != nil {
						if err != io.EOF {
							fmt.Println("failed to read plate size", err)

						}
						return
					}

					plate := make([]byte, plateSize[0])
					_, err = io.ReadFull(client, plate)
					if err != nil {
						if err != io.EOF {
							fmt.Println("failed to read plate", err)

						}
						return
					}

					tsB := make([]byte, 4)
					_, err = io.ReadFull(client, tsB)
					if err != nil {
						if err != io.EOF {
							fmt.Println("failed to read plate timestamp", err)
						}
						return
					}
					ts := binary.BigEndian.Uint32(tsB)

					t.plate(road, mile, limit, string(plate), ts)

				case WantHeartbeat:
					if handleHeartbeatRequest(ctx, client) {
						return
					}
				default:
					rsp := errorMsg(msgType)
					_, err := client.Write(rsp)
					if err != nil {
						if err != io.EOF {
							fmt.Println("failed to send error to client", err)
						}
						return
					}
					return
				}
			}
		case IAmDispatcher:
			nRoadsB := make([]byte, 1)
			_, err = io.ReadFull(client, nRoadsB)
			if err != nil {
				if err != io.EOF {
					fmt.Println("failed to read number of roads", err)
				}
				return
			}

			roadsB := make([]byte, 2*nRoadsB[0])
			_, err = io.ReadFull(client, roadsB)
			if err != nil {
				if err != io.EOF {
					fmt.Println("failed to read plate timestamp", err)
				}
				return
			}

			roadCount := int(nRoadsB[0])

			r := make([]uint16, roadCount)
			if roadCount > 0 {
				for i := 0; i < roadCount; i++ {
					r[0] = binary.BigEndian.Uint16(roadsB[2*i : 2*i+2])
				}
			}

			ticketCh := t.registerDispatcher(r)
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case t := <-ticketCh:
						ticketB := make([]byte, 1+1+len(t.plate)+2+2+4+2+4+2)
						i := 0
						ticketB[i] = Ticket
						i++
						ticketB[i] = uint8(len(t.plate))
						i++
						copy(ticketB[i:i+len(t.plate)], t.plate)
						i += len(t.plate)
						binary.BigEndian.PutUint16(ticketB[i:i+2], t.road)
						i += 2

						if t.ts1 < t.ts2 {
							binary.BigEndian.PutUint16(ticketB[i:i+2], t.mile1)
							i += 2
							binary.BigEndian.PutUint32(ticketB[i:i+4], t.ts1)
							i += 4

							binary.BigEndian.PutUint16(ticketB[i:i+2], t.mile2)
							i += 2
							binary.BigEndian.PutUint32(ticketB[i:i+4], t.ts2)
							i += 4

						} else {
							binary.BigEndian.PutUint16(ticketB[i:i+2], t.mile2)
							i += 2
							binary.BigEndian.PutUint32(ticketB[i:i+4], t.ts2)
							i += 4

							binary.BigEndian.PutUint16(ticketB[i:i+2], t.mile1)
							i += 2
							binary.BigEndian.PutUint32(ticketB[i:i+4], t.ts1)
							i += 4
						}

						binary.BigEndian.PutUint16(ticketB[i:i+2], t.speed*100)

						fmt.Printf("ticket speed (x100): %d\n", t.speed*100)

						_, err = client.Write(ticketB)
						if err != nil {
							if err != io.EOF {
								fmt.Println("failed to send ticket", err)
							}
							return
						}
					}
				}
			}()

		case WantHeartbeat:
			if handleHeartbeatRequest(ctx, client) {
				return
			}

		default:
			rsp := errorMsg(msgType)
			_, err := client.Write(rsp)
			if err != nil {
				if err != io.EOF {
					fmt.Println("failed to send error to client", err)
				}
				return
			}
			return
		}
	}
}

func handleHeartbeatRequest(ctx context.Context, client net.Conn) bool {
	ib := make([]byte, 4)
	_, err := io.ReadFull(client, ib)
	if err != nil {
		if err != io.EOF {
			fmt.Println("failed to read heartbeat interval", err)
		}
		return true
	}

	i := binary.BigEndian.Uint32(ib)

	if i == 0 {
		return false
	}

	go func() {
		t := time.NewTicker(time.Duration(i) * 100 * time.Millisecond)

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_, err := client.Write([]byte{0x41})
				if err != nil {
					if err != io.EOF {
						fmt.Println("failed to send heartbeat", err)
					}
					return
				}
			}
		}
	}()
	return false
}

func errorMsg(msgType []byte) []byte {
	msg := fmt.Sprintf("invalid message: %02x", msgType[0])
	rsp := make([]byte, 1+len(msg))
	rsp[0] = uint8(len(msg))
	copy(rsp[1:], []byte(msg))
	return rsp
}

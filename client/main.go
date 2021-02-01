package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/pb"
	"github.com/joho/godotenv"
	"github.com/warthog618/gpiod"
)

const (
	defaultAddress = "192.168.0.201:20000"
)

var dotEnv map[string]string

// Watches GPIO buttons and notifies server when their state changes
func main() {
	// Set up a connection to the server.
	address := defaultAddress
	if len(os.Args) > 1 {
		address = fmt.Sprintf("192.168.0.%s:20000", os.Args[1])
	}

	chip, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}

	// Find out which pins we're working with
	dotEnv, e := godotenv.Read()
	if e != nil {
		panic(e)
	}
	redButton, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
	fmt.Printf("Found pin %d for red button in .env ...\n", redButton)
	blackButton, _ := strconv.Atoi(dotEnv["BLACK_BUTTON_PIN"])
	fmt.Printf("Found pin %d for black button in .env ...\n", blackButton)

	fmt.Println("dialing", address)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	fmt.Println("dialed")
	defer conn.Close()
	client := pb.NewReceiverClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := client.ButtonState(ctx)
	fmt.Println("created client")

	replyCh := make(chan string)

	fmt.Println("instantiating closure")
	eventHandler := eventClosure(stream, replyCh)
	fmt.Println("requesting line")
	lines, err := chip.RequestLines([]int{redButton, blackButton},
		gpiod.WithDebounce(time.Millisecond*50),
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(eventHandler))
	defer lines.Close()
	fmt.Println("requested both lines")
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithDebounce option requires kernel V5.10 or later - check your kernel version.")
		}
		os.Exit(1)
	}

	fmt.Printf("Watching Pin %d...\n", redButton)
	fmt.Printf("Watching Pin %d...\n", blackButton)
	// Wait for lines to be closed and server response
	reply := <-replyCh
	log.Printf("reply: %v", reply)
}

// Event handler needs access to the pb client, but the args can't be modified
// so the closure adds it to the scope
func eventClosure(stream pb.Receiver_ButtonStateClient, replyCh chan string) func(gpiod.LineEvent) {
	// not sure why this needs to be read again
	dotEnv, e := godotenv.Read()
	if e != nil {
		panic(e)
	}
	fmt.Println("read env")
	closed := false
	return func(evt gpiod.LineEvent) {
		t := time.Now()
		pressed := false
		if evt.Type == gpiod.LineEventFallingEdge {
			pressed = true
		}
		fmt.Printf("event:%3d %v, %-7s %s (%s)\n",
			evt.Offset,
			evt.Type,
			pressed,
			t.Format(time.RFC3339Nano),
			evt.Timestamp)
		redPin, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
		button := "black"
		if evt.Offset == redPin {
			button = "red"
		}
		if !closed {
			stream.Send(&pb.ButtonStateChange{Button: button, Pressed: pressed})
			if evt.Offset == redPin && pressed {
				// Stop processing additional events
				closed = true
				log.Printf("received red button, ending")
				log.Printf("close and receive")
				msg, err := stream.CloseAndRecv()
				if err != nil {
					log.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
				}
				replyCh <- msg.Message
				return
			}
		}
	}
}

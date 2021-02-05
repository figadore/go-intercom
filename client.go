package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jfreymuth/pulse"
	"github.com/warthog618/gpiod"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/pb"
)

// Watches GPIO buttons and notifies server when their state changes
func createClient(server string) {
	// Set up a connection to the server.
	address := fmt.Sprintf("192.168.0.%s%s", server, port)

	log.Println("dialing", address)
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	log.Println("dialed")
	defer conn.Close()
	client := pb.NewReceiverClient(conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer log.Println("canceling btnstate")
	defer cancel()

	stream, err := client.ButtonState(ctx)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	replyCh := make(chan error)
	eventHandler := eventClosure(stream, replyCh)
	lines := initializeButtons(eventHandler)
	defer lines.Close()

	audioCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	audioStream, err := client.Audio(audioCtx)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	errCh := make(chan error)
	if len(os.Args) > 2 {
		go startStreaming(audioCtx, audioStream, errCh)
	}

	select {
	case err = <-errCh:
		if err != nil {
			if err != pulse.EndOfData {
				log.Fatalf("Error occurred: %v, exiting", err)
			}
		}
	case err = <-replyCh:
		if err != nil {
			log.Fatalf("Error occurred: %v, exiting", err)
		}
	}
}

// Event handler needs access to the pb client and a reply
// channel, but the args can't be modified so the closure
// adds them to the scope
func eventClosure(stream pb.Receiver_ButtonStateClient, replyCh chan error) func(gpiod.LineEvent) {
	return func(evt gpiod.LineEvent) {
		t := time.Now()
		pressed := false
		if evt.Type == gpiod.LineEventFallingEdge {
			pressed = true
		}
		log.Printf("event:%3d, %v, %v, %-7s (%s)\n",
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
		stateChange := &pb.ButtonStateChange{Button: button, Pressed: pressed}
		err := stream.Send(stateChange)
		if err != nil {
			log.Printf("%v.Send() got error %v, want %v", stream, err, nil)
			replyCh <- err
		} else {
			// Change local LED if LED send was a success so client at least knows the button does something
			err = updateLeds(stateChange)
			if err != nil {
				log.Printf("%v.updateLeds() got error %v, want %v", stream, err, nil)
				replyCh <- err
			}
		}
	}
}

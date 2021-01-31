/*
 *
 * Copyright 2015 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package main implements a client for Greeter service.
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

	eventHandler := eventClosure(stream, replyCh)
	red, err := chip.RequestLine(redButton,
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(eventHandler))
	fmt.Println("requested first line")
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPullUp option requires kernel V5.5 or later - check your kernel version.")
		}
		os.Exit(1)
	}
	black, err := chip.RequestLine(blackButton,
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(eventHandler))
	fmt.Println("requested second line")
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPullUp option requires kernel V5.5 or later - check your kernel version.")
		}
		os.Exit(1)
	}

	fmt.Printf("Watching Pin %d...\n", redButton)
	fmt.Printf("Watching Pin %d...\n", blackButton)
	<-replyCh
	log.Printf("closing red line")
	red.Close()
	log.Printf("closing black line")
	black.Close()
	log.Printf("close and receive")
	msg, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	log.Printf("reply: %v", msg.Message)
}

func eventClosure(stream pb.Receiver_ButtonStateClient, replyCh chan string) func(gpiod.LineEvent) {
	dotEnv, e := godotenv.Read()
	if e != nil {
		panic(e)
	}
	return func(evt gpiod.LineEvent) {
		t := time.Now()
		pressed := false
		if evt.Type == gpiod.LineEventFallingEdge {
			pressed = true
		}
		fmt.Printf("event:%3d %-7s %s (%s)\n",
			evt.Offset,
			pressed,
			t.Format(time.RFC3339Nano),
			evt.Timestamp)
		redPin, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
		log.Printf("button %v (%T), redPin %v (%T)", evt.Offset, evt.Offset, redPin, redPin)
		// blackPin, _ := strconv.Atoi(dotEnv["BLACK_BUTTON_PIN"])
		button := "black"
		if evt.Offset == redPin {
			button = "red"
		}
		err := stream.Send(&pb.ButtonStateChange{Button: button, Pressed: pressed})
		if err != nil {
			log.Fatalf("could not send button state change: %v", err)
		}
		if evt.Offset == redPin && pressed {
			log.Printf("received red button, ending")
			replyCh <- "ok"
		}
	}
}

// SPDX-License-Identifier: MIT

// A simple example that watches an input pin and reports edge events.
package main

import (
	"log"
	"os"

	"github.com/warthog618/gpiod"
)

const (
	port = ":20000"
)

var (
	dotEnv              map[string]string
	greenLed, yellowLed *gpiod.Line
)

func main() {
	chip, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}
	defer chip.Close()
	initializeLeds(chip)
	defer greenLed.Close()
	defer yellowLed.Close()

	done := make(chan bool)
	go startServer(done)

	log.Printf("Start with %v args", len(os.Args))
	log.Printf("Arg 0: %v", os.Args[0])
	log.Printf("Arg 1: %v", os.Args[1])
	// Create a client for each ip address passed in
	for _, serverAddr := range os.Args[1:] {
		// Add retry logic, can't start client until server is up
		go createClient(serverAddr)
	}
	// Wait for server exit
	<-done
	log.Println("Exiting successfully")
}

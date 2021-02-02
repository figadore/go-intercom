// SPDX-License-Identifier: MIT

// A simple example that watches an input pin and reports edge events.
package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/warthog618/gpiod"
)

func eventHandler(evt gpiod.LineEvent) {
	t := time.Now()
	edge := "rising"
	if evt.Type == gpiod.LineEventFallingEdge {
		edge = "falling"
	}
	fmt.Printf("event:%3d %-7s %s (%s)\n",
		evt.Offset,
		edge,
		t.Format(time.RFC3339Nano),
		evt.Timestamp)
}

// Watches GPIO 8 and 20 and reports when they changes state.
func main() {
	c, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	var dotEnv map[string]string
	dotEnv, e := godotenv.Read()
	if e != nil {
		panic(e)
	}
	redButton, _ := strconv.Atoi(dotEnv["RED_BUTTON_PIN"])
	blackButton, _ := strconv.Atoi(dotEnv["BLACK_BUTTON_PIN"])
	fmt.Printf("Found pin %d for black button in .env ...\n", blackButton)
	r, err := c.RequestLine(redButton,
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(eventHandler))
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPullUp option requires kernel V5.5 or later - check your kernel version.")
		}
		os.Exit(1)
	}
	b, err := c.RequestLine(blackButton,
		gpiod.WithBothEdges,
		gpiod.WithEventHandler(eventHandler))
	if err != nil {
		fmt.Printf("RequestLine returned error: %s\n", err)
		if err == syscall.Errno(22) {
			fmt.Println("Note that the WithPullUp option requires kernel V5.5 or later - check your kernel version.")
		}
		os.Exit(1)
	}
	defer r.Close()
	defer b.Close()

	// In a real application the main thread would do something useful.
	// But we'll just run for a minute then exit.
	fmt.Printf("Watching Pin %d...\n", redButton)
	fmt.Printf("Watching Pin %d...\n", blackButton)
	time.Sleep(time.Minute)
	fmt.Println("exiting...")
}

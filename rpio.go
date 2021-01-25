package main

//gp-rpio library causes freezing to occur during edge detection. see https://github.com/stianeikeland/go-rpio/issues/46

import (
    "os"
    "fmt"
    "time"

    "github.com/stianeikeland/go-rpio"
)

func run() {
	// Open and map memory to access gpio, check for errors
	if err := rpio.Open(); err != nil {
		//panic(fmt.Sprint("unable to open gpio", err.Error()))
		fmt.Println(err)
		os.Exit(1)
	}
	// Unmap gpio memory when done
	defer rpio.Close()

    greenLed := rpio.Pin(2)
    yellowLed := rpio.Pin(3)
    blackButton := rpio.Pin(20)
    redButton := rpio.Pin(8)
    greenLed.Output()
    yellowLed.Output()
    blackButton.Input()
    redButton.Input()

	redButton.Detect(rpio.FallEdge) // enable falling edge event detection
	blackButton.Detect(rpio.FallEdge) // enable falling edge event detection

	fmt.Println("press a button")

	for i := 0; i < 2; {
		if blackButton.EdgeDetected() { // check if event occured
			greenLed.Toggle()
			i++
		}
		time.Sleep(time.Second / 2)
	}
	blackButton.Detect(rpio.NoEdge) // disable edge event detection
    //for x := 0; x < 20; x++ {
    //    greenLed.Toggle()
    //    yellowLed.Toggle()
    //    time.Sleep(time.Second / 5)
    //}
}

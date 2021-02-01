package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/figadore/go-intercom/pb"
	"github.com/joho/godotenv"
	"github.com/warthog618/gpiod"
	"google.golang.org/grpc"
)

const (
	port = ":20000"
)

var greenLed, yellowLed *gpiod.Line

func main() {
	log.Printf("Booting")
	fmt.Println("up")
	lis, err := net.Listen("tcp", port)
	fmt.Println("listening")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	fmt.Println("new server")
	pb.RegisterReceiverServer(s, &server{})
	fmt.Println("registered")

	// Find out which pins we're working with
	dotEnv, e := godotenv.Read()
	if e != nil {
		panic(e)
	}
	greenLedPin, _ := strconv.Atoi(dotEnv["GREEN_LED_PIN"])
	fmt.Printf("Found pin %d for green LEDn .env ...\n", greenLedPin)
	yellowLedPin, _ := strconv.Atoi(dotEnv["YELLOW_LED_PIN"])
	fmt.Printf("Found pin %d for yellow LED in .env ...\n", yellowLedPin)

	chip, err := gpiod.NewChip("gpiochip0")
	if err != nil {
		panic(err)
	}
	greenLed, err = chip.RequestLine(greenLedPin, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
	defer greenLed.Close()
	yellowLed, err = chip.RequestLine(yellowLedPin, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
	defer yellowLed.Close()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	log.Printf("Started serve on port: %v", port)
}

// "inherit" from unimplemented for future compatibility
type server struct {
	pb.UnimplementedReceiverServer
}

func (s *server) ButtonState(stream pb.Receiver_ButtonStateServer) error {
	log.Printf("ButtonState entered")
	count := 0
	for {
		stateChange, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Client disconnected")
			return stream.SendAndClose(&pb.ButtonChangeReply{
				Message: fmt.Sprintf("success? %v state changes received", count),
			})
		}
		if err != nil {
			return err
		}
		if stateChange.GetButton() == "black" {
			if stateChange.GetPressed() {
				err = greenLed.SetValue(1)
			} else {
				err = greenLed.SetValue(0)
			}
		} else {
			if stateChange.GetPressed() {
				err = yellowLed.SetValue(1)
			} else {
				err = yellowLed.SetValue(0)
			}
		}
		if err != nil {
			log.Fatalf("Unable to set LED value for button %s and pressed %s", stateChange.GetButton(), strconv.FormatBool(stateChange.GetPressed()))
		}
		log.Printf("Received: %v from %v", stateChange.GetPressed(), stateChange.GetButton())
		count++
	}
}

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/figadore/go-intercom/pb"
	"google.golang.org/grpc"
)

func startServer(done chan bool) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterReceiverServer(s, &server{})

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	log.Printf("Serving on port: %v", port)
	done <- true
}

// "inherit" from unimplemented for future compatibility
type server struct {
	pb.UnimplementedReceiverServer
}

// ButtonState client streaming RPC
func (s *server) ButtonState(stream pb.Receiver_ButtonStateServer) error {
	log.Printf("ButtonState gRPC client connected")
	stateChangeCount := 0
	for {
		stateChange, err := stream.Recv()
		if err == io.EOF {
			log.Printf("ButtonState gRPC client disconnected")
			return stream.SendAndClose(&pb.ButtonChangeReply{
				Message: fmt.Sprintf("success? %v state changes received", stateChangeCount),
			})
		} else if err != nil {
			return err
		}
		err = updateLeds(stateChange)
		if err != nil {
			log.Fatalf("Unable to set LED value for button %s and pressed %s", stateChange.GetButton(), strconv.FormatBool(stateChange.GetPressed()))
		}
		log.Printf("Received: %v from %v", stateChange.GetPressed(), stateChange.GetButton())
		stateChangeCount++
	}
}

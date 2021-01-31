package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/pb"
)

const (
	port = ":20000"
)

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
			return stream.SendAndClose(&pb.ButtonChangeReply{
				Message: fmt.Sprintf("success? %v state changes received", count),
			})
		}
		if err != nil {
			return err
		}
		log.Printf("Received: %v from %v", stateChange.GetPressed(), stateChange.GetButton())
		count++
	}
}

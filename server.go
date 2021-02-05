package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/figadore/go-intercom/pb"
	"github.com/jfreymuth/pulse"
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

func (s *server) Audio(clientStream pb.Receiver_AudioServer) error {
	c, err := pulse.NewClient()
	if err != nil {
		return err
	}
	defer c.Close()
	playableBuffer := audioBuffer{
		stream: clientStream,
	}
	for {
		bufferReader := readBufferClosure(&playableBuffer)
		pulseStream, err := c.NewPlayback(pulse.Float32Reader(bufferReader), pulse.PlaybackLatency(.1))
		if err != nil {
			return err
		}
		pulseStream.Start()
		pulseStream.Drain()
		fmt.Println("Underflow:", pulseStream.Underflow())
		if pulseStream.Error() != nil {
			fmt.Println("Error:", pulseStream.Error())
		}
		pulseStream.Close()

		//if err := clientStream.Send(note); err != nil {
		//	return err
		//}
	}
}

type audioBuffer struct {
	sync.Mutex
	bytes  []float32
	stream pb.Receiver_AudioServer
}

func (b *audioBuffer) fill() error {
	receivedBytes, err := b.stream.Recv()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	b.bytes = append(b.bytes, receivedBytes.Data...)
	return nil
}

// Give readBuffer function access to playableBuffer
func readBufferClosure(playableBuffer *audioBuffer) func([]float32) (int, error) {
	return func(speakerBytes []float32) (int, error) {
		// Copy buffer so we can unlock and other processes can access it
		playableBuffer.Lock()
		bytesCopied := len(playableBuffer.bytes)
		if bytesCopied == 0 {
			// blocking
			err := playableBuffer.fill()
			if err != nil {
				return 0, err
			}
		}
		// tmp := make([]float32, len(playableBuffer.bytes))
		// copy(tmp, playableBuffer)
		// playableBuffer.Unlock()

		if len(speakerBytes) < bytesCopied {
			bytesCopied = len(speakerBytes)
		}
		// There's probably a better way to do this
		// something like "speakerBytes[:bytesCopied] = playableBuffer.bytes[:bytesCopied]"
		for i := 0; i < bytesCopied; i++ {
			speakerBytes[i] = playableBuffer.bytes[i]
		}
		// playableBuffer.Lock()
		playableBuffer.bytes = playableBuffer.bytes[bytesCopied:]
		playableBuffer.Unlock()
		return bytesCopied, nil
	}
}

package rpc

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/jfreymuth/pulse"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/rpc/pb"
)

//type Server struct {
//	station *station.Station
//	server  *server
//}

func NewServer() *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterIntercomServer(s, &Server{})
	return s
}

func Serve(s *grpc.Server, errCh chan error) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Printf("failed to listen: %v", err)
		errCh <- err
	}
	if err := s.Serve(lis); err != nil {
		log.Printf("failed to serve: %v", err)
		errCh <- err
	}
}

// "inherit" from unimplemented for future compatibility
type Server struct {
	pb.UnimplementedIntercomServer
}

func (s *Server) DuplexCall(clientStream pb.Intercom_DuplexCallServer) error {
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

type audioBuffer struct {
	sync.Mutex
	bytes  []float32
	stream pb.Intercom_DuplexCallServer
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

package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/figadore/go-intercom/internal/log"
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
	log.Debugf("Start net.Listen: %v", port)
	defer log.Debugf("Finished net.Listen: %v", port)
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
	log.Debugln("Start server side DuplexCall")
	c, err := pulse.NewClient()
	if err != nil {
		return err
	}
	defer log.Debugln("Server DuplexCall: Closed pulse client")
	defer c.Close()
	defer log.Debugln("Server DuplexCall: Closing pulse client")
	//playableBuffer := audioBuffer{
	//	stream: clientStream,
	//}
	//bufferReader := readBufferClosure(&playableBuffer)
	//speakerStream, err := c.NewPlayback(pulse.Float32Reader(bufferReader), pulse.PlaybackLatency(.1))

	bytesCh := make(chan float32, 256)
	b := pulseCallReader{
		pulseClient: c,
		bytesCh:     bytesCh,
	}
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer log.Debugln("Server DuplexCall: Cancelled server duplex call context")
	defer cancel()
	defer log.Debugln("Server DuplexCall: Cancelling server duplex call context")
	go startServerReceiving(ctx, clientStream, bytesCh, errCh)
	speakerStream, err := c.NewPlayback(pulse.Float32Reader(b.Read), pulse.PlaybackLatency(.1))
	if err != nil {
		log.Println("Server DuplexCall: Unable to create new pulse recorder", err)
		return err
	}
	log.Debugln("Server DuplexCall: Starting speaker stream")
	speakerStream.Start()
	log.Debugln("Server DuplexCall: Started speaker stream")
	<-errCh
	log.Debugln("Server DuplexCall: Error channel for server receiving go routing got error or closed")
	speakerStream.Drain()
	fmt.Println("Server DuplexCall: Drained. Underflow:", speakerStream.Underflow())
	if speakerStream.Error() != nil {
		fmt.Println("Error:", speakerStream.Error())
	}
	speakerStream.Close()
	log.Debugln("Server DuplexCall: speakerStream closed")
	return nil
}

type pulseCallReader struct {
	pulseClient *pulse.Client
	// callServer  pb.Intercom_DuplexCallServer
	bytesCh chan float32
}

func startServerReceiving(ctx context.Context, stream pb.Intercom_DuplexCallServer, bytesCh chan float32, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("startServerReceiving: Context.Done: %v", ctx.Err())
			errCh <- ctx.Err()
			return

		default:
			log.Debugln("startServerReceiving: looping")
			bytes, err := stream.Recv()
			if err == io.EOF {
				log.Println("startServerReceiving: Received end of data from incoming connection")
				// errCh <- nil
			}
			if err != nil {
				log.Println("startServerReceiving: Non-nil error", err)
				errCh <- err
				return
			}
			// bytesCh <- bytes.Data
			for _, b := range bytes.Data {
				log.Println("startServerReceiving: sending to bytesCh")
				bytesCh <- b
				log.Println("startServerReceiving: sent to bytesCh")
			}
		}
		//}
	}
}

func (b pulseCallReader) Read(p []float32) (n int, err error) {
	log.Debugln("pulseCallReader.Read: begin")
	defer log.Debugf("pulseCallReader.Read: end. %v bytes", n)
	for n = 0; n < len(p); n++ {
		sample, ok := <-b.bytesCh
		if !ok {
			err = errors.New("Server pulse call reader: Bytes channel appears closed")
			return
		}
		p[n] = sample
	}
	//for n = 0; n < len(p); n++ {
	//	select {
	//	case b, ok := <-b.bytesCh:
	//		if !ok {
	//			err = errors.New("Server pulse call reader: Bytes channel appears closed")
	//			return
	//		}
	//		p[n] = b
	//	default:
	//		log.Printf("Pulse call reader buffered channel appears empty, returning after %v bytes", n)
	//		return
	//	}
	//}
	//if len(p) < len(receivedBytes.Data) {
	//	// cn't fill reader buffer with all received data
	//	// TODO use buffered channels
	//	// also, apply to writer?
	//}
	//for i := 0; i < len(receivedBytes.Data); i++ {
	//	p[i] = receivedBytes.Data[i]
	//	n = i
	//}
	return
}

//// Give readBuffer function access to playableBuffer
//func readBufferClosure(playableBuffer *audioBuffer) func([]float32) (int, error) {
//	return func(speakerBytes []float32) (int, error) {
//		// Copy buffer so we can unlock and other processes can access it
//		playableBuffer.Lock()
//		bytesCopied := len(playableBuffer.bytes)
//		if bytesCopied == 0 {
//			// blocking
//			err := playableBuffer.fill()
//			if err != nil {
//				return 0, err
//			}
//		}
//		// tmp := make([]float32, len(playableBuffer.bytes))
//		// copy(tmp, playableBuffer)
//		// playableBuffer.Unlock()
//
//		if len(speakerBytes) < bytesCopied {
//			bytesCopied = len(speakerBytes)
//		}
//		// There's probably a better way to do this
//		// something like "speakerBytes[:bytesCopied] = playableBuffer.bytes[:bytesCopied]"
//		for i := 0; i < bytesCopied; i++ {
//			speakerBytes[i] = playableBuffer.bytes[i]
//		}
//		// playableBuffer.Lock()
//		playableBuffer.bytes = playableBuffer.bytes[bytesCopied:]
//		playableBuffer.Unlock()
//		return bytesCopied, nil
//	}
//}
//
//type audioBuffer struct {
//	sync.Mutex
//	bytes  []float32
//	stream pb.Intercom_DuplexCallServer
//}
//
//func (b *audioBuffer) fill() error {
//	receivedBytes, err := b.stream.Recv()
//	if err == io.EOF {
//		return nil
//	}
//	if err != nil {
//		return err
//	}
//	b.bytes = append(b.bytes, receivedBytes.Data...)
//	return nil
//}

package rpc

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sync"

	"github.com/jfreymuth/pulse"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/rpc/pb"
	"github.com/figadore/go-intercom/internal/station"
	"github.com/figadore/go-intercom/pkg/call"
)

type grpcCallManager struct {
	callList []call.Call
	station  *station.Station
}

// add common methods here
func (p *grpcCallManager) EndCalls() {
	for _, call := range p.callList {
		call.Hangup()
	}
}

func NewCallManager() *grpcCallManager {
	return &grpcCallManager{
		callList: make([]call.Call, 0),
	}
}

func (p *grpcCallManager) SetStation(s *station.Station) {
	p.station = s
}

func (p *grpcCallManager) outgoingCall(parentContext context.Context, address string) {
	fullAddress := fmt.Sprintf("192.168.0.%s%s", address, port)

	err := p.station.Outputs.OutgoingCall(parentContext, address)
	if err != nil {
		msg := fmt.Sprintf("Warning: Unable to inform station outputs of outgoing call %v: %v", fullAddress, err)
		log.Println(msg)
		return
	}

	// Set up a connection to the server.

	log.Println("dialing", fullAddress)
	conn, err := grpc.Dial(fullAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		msg := fmt.Sprintf("Warning: Unable to dial %v: %v", fullAddress, err)
		log.Println(msg)
		return
	}
	log.Println("dialed")
	defer conn.Close()
	client := pb.NewIntercomClient(conn)
	ctx, cancel := context.WithCancel(parentContext)
	defer log.Printf("canceling outgoing call to %v in defer", fullAddress)
	defer cancel()

	stream, err := client.DuplexCall(ctx)
	if err != nil {
		log.Printf("Failed to start grpc client listener: %v", err)
		return
	}
	replyCh := make(chan error)

	errCh := make(chan error)

	go startStreaming(ctx, stream, errCh)
	go startReceiving(ctx, stream, errCh)

	select {
	case <-parentContext.Done():
		log.Printf("Parent context Done() with err: %v", err)
		return
	case err = <-errCh:
		if err != nil {
			if err != pulse.EndOfData {
				log.Fatalf("Error occurred: %v, exiting", err)
			}
		}
	case err = <-replyCh:
		if err != nil {
			log.Fatalf("Error occurred: %v, exiting", err)
		}
	}
}

func (p *grpcCallManager) CallAll(ctx context.Context) {
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		p.outgoingCall(ctx, address)
	}
}

func startReceiving(ctx context.Context, stream pb.Intercom_DuplexCallClient, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context.Done: %v", ctx.Err())
			errCh <- ctx.Err()

		default:
			// TODO play received audio
			_, err := stream.Recv()
			if err == io.EOF {
				log.Println("Received end of data from incoming connection")
				// errCh <- nil
				return
			}
			if err != nil {
				errCh <- err
			}
		}
	}
}

func startStreaming(ctx context.Context, stream pb.Intercom_DuplexCallClient, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context.Done: %v", ctx.Err())
			errCh <- ctx.Err()

		default:
			i, err := sendAudio(stream)
			if err != nil {
				if err == pulse.EndOfData {
					log.Printf("End of streaming data: %v bytes", i)
				} else {
					errCh <- err
				}
			}
		}
	}
}

type sendBuffer struct {
	sync.Mutex
	bytes []float32
}

var t, phase float32

func (b *sendBuffer) fill() (int, error) {
	for i := range b.bytes {
		if t > 4 {
			return i, pulse.EndOfData
		}
		x := float32(math.Sin(2 * math.Pi * float64(phase)))
		b.bytes[i] = x * 0.05
		f := [...]float32{440, 550, 440, 880}[int(2*t)&3]
		phase += f / 44100
		if phase >= 1 {
			phase--
		}
		t += 1. / 44100
	}
	return len(b.bytes), nil
}

func sendAudio(stream pb.Intercom_DuplexCallClient) (int, error) {
	micBuffer := sendBuffer{
		bytes: make([]float32, 256),
	}
	i, err := micBuffer.fill()
	if err != nil {
		return i, err
	}
	bytes := pb.AudioData{
		Data: micBuffer.bytes,
	}
	err = stream.Send(&bytes)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

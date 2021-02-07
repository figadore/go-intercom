package rpc

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

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
		log.Printf("Parent context Done() with err: %v", parentContext.Err())
		return
	case <-ctx.Done():
		log.Printf("Current context Done() with err: %v", ctx.Err())
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
			return

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
				return
			}
		}
	}
}

// func Write(p []byte) (n int, err error)

func startStreaming(ctx context.Context, stream pb.Intercom_DuplexCallClient, errCh chan error) {
	c, err := pulse.NewClient()
	if err != nil {
		errCh <- err
		return
	}
	defer c.Close()
	b := sendBuffer{
		pulseClient: c,
		callClient:  stream,
	}
	micStream, err := c.NewRecord(pulse.Float32Writer(b.Write))
	if err != nil {
		log.Println("Unable to create new pulse recorder", err)
		errCh <- err
	}
	micStream.Start()
	defer micStream.Stop()
	<-ctx.Done()
	log.Printf("Context.Done: %v", ctx.Err())
	errCh <- ctx.Err()
}

type sendBuffer struct {
	pulseClient *pulse.Client
	callClient  pb.Intercom_DuplexCallClient
}

func (b sendBuffer) Write(p []float32) (n int, err error) {
	c := b.callClient
	data := pb.AudioData{
		Data: p,
	}
	err = c.Send(&data)
	n = len(p)
	return
}

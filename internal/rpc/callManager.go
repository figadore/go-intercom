package rpc

import (
	"context"
	"fmt"
	//"io"
	"os"

	"github.com/jfreymuth/pulse"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
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
	log.Debugln("outgoingCall: Start client side DuplexCall")
	select {
	case <-parentContext.Done():
		msg := fmt.Sprintf("outgoingCall: Error: parent context cancelled: %v", parentContext.Err())
		log.Println(msg)
		return
	default:
	}
	ctx, cancel := context.WithCancel(parentContext)
	defer cancel()
	errCh := make(chan error)
	audioInCh := make(chan float32, 256)
	audioOutCh := make(chan float32, 256)

	// Initiate a grpc connection with the server
	fullAddress := fmt.Sprintf("192.168.0.%s%s", address, port)
	log.Println("dialing", fullAddress)
	conn, err := grpc.Dial(fullAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		msg := fmt.Sprintf("Warning: Unable to dial %v: %v", fullAddress, err)
		log.Println(msg)
		return
	}
	log.Debugln("dialed")
	defer conn.Close()
	client := pb.NewIntercomClient(conn)
	serverStream, err := client.DuplexCall(ctx)
	if err != nil {
		log.Printf("placeCall: error creating duplex call client: %v", err)
		return
	}

	go startReceiving(ctx, audioInCh, errCh, serverStream.Recv)
	go startPlayback(ctx, audioInCh, errCh)
	go startRecording(ctx, audioOutCh, errCh)
	go startSending(ctx, audioOutCh, errCh, serverStream.Send)
	select {
	case <-ctx.Done():
		log.Printf("placeCall: context.Done: %v", ctx.Err())
		return
	case err := <-errCh:
		log.Printf("placeCall: errCh: %v", err)
		return
	}
}

func startReceiving(ctx context.Context, audioCh chan float32, errCh chan error, recvFn func() (*pb.AudioData, error)) {
}

func startSending(ctx context.Context, audioCh chan float32, errCh chan error, sendFn func(*pb.AudioData) error) {
}

func startPlayback(ctx context.Context, audioCh chan float32, errCh chan error) {
	c, err := pulse.NewClient()
	if err != nil {
		errCh <- err
	}
	defer c.Close()
}

func startRecording(ctx context.Context, audioCh chan float32, errCh chan error) {
	c, err := pulse.NewClient()
	if err != nil {
		errCh <- err
	}
	defer c.Close()
}

//func (p *grpcCallManager) outgoingCall(parentContext context.Context, address string) {
//	log.Debugln("callManager.outgoingCall: enter")
//	defer log.Debugln("callManager.outgoingCall: exit")
//	fullAddress := fmt.Sprintf("192.168.0.%s%s", address, port)
//
//	err := p.station.Outputs.OutgoingCall(parentContext, address)
//	if err != nil {
//		msg := fmt.Sprintf("Warning: Unable to inform station outputs of outgoing call %v: %v", fullAddress, err)
//		log.Println(msg)
//		return
//	}
//
//	// Set up a connection to the server.
//
//	log.Println("dialing", fullAddress)
//	conn, err := grpc.Dial(fullAddress, grpc.WithInsecure(), grpc.WithBlock())
//	if err != nil {
//		msg := fmt.Sprintf("Warning: Unable to dial %v: %v", fullAddress, err)
//		log.Println(msg)
//		return
//	}
//	log.Debugln("dialed")
//	defer conn.Close()
//	client := pb.NewIntercomClient(conn)
//	ctx, cancel := context.WithCancel(parentContext)
//	defer log.Debugf("callManager.outoingCall: cancelled outgoing call to %v in defer", fullAddress)
//	defer cancel()
//	defer log.Debugf("callManager.outoingCall: cancelling outgoing call to %v in defer", fullAddress)
//
//	stream, err := client.DuplexCall(ctx)
//	if err != nil {
//		log.Printf("Failed to start grpc client listener: %v", err)
//		return
//	}
//	replyCh := make(chan error)
//
//	errCh := make(chan error)
//
//	log.Debugln("callManager.outoingCall: starting streaming go routine")
//	go startStreaming(ctx, stream, errCh)
//	bytesCh := make(chan float32, 256)
//	log.Debugln("callManager.outoingCall: starting receiving go routine")
//	go startReceivings(ctx, stream, bytesCh, errCh)
//
//	// setup speaker as receiver for data from server
//	c, err := pulse.NewClient()
//	if err != nil {
//		log.Println(err)
//		return
//	}
//	defer log.Debugln("callManager.outoingCall: closed pulse client")
//	defer c.Close()
//	defer log.Debugln("callManager.outoingCall: closing pulse client")
//	b := pulseCallReader{
//		pulseClient: c,
//		bytesCh:     bytesCh,
//	}
//	speakerStream, err := c.NewPlayback(pulse.Float32Reader(b.Read), pulse.PlaybackLatency(.1))
//	if err != nil {
//		log.Println("Unable to create new pulse recorder", err)
//		return
//	}
//	log.Debugln("callManager.outoingCall: start speakerStream")
//	go speakerStream.Start()
//	defer speakerStream.Close()
//	defer log.Debugln("callManager.outoingCall: drained speakerStream")
//	defer speakerStream.Drain()
//	defer log.Debugln("callManager.outoingCall: draining speakerStream")
//	if speakerStream.Error() != nil {
//		log.Println("Error:", speakerStream.Error())
//		return
//	}
//
//	log.Debugln("callManager.outoingCall: channel select")
//	select {
//	case <-parentContext.Done():
//		log.Printf("Parent context Done() with err: %v", parentContext.Err())
//		return
//	case <-ctx.Done():
//		log.Printf("Current context Done() with err: %v", ctx.Err())
//		return
//	case err = <-errCh:
//		log.Debugln("callManager.outoingCall: channel select")
//		if err != nil {
//			if err != pulse.EndOfData {
//				log.Printf("Error occurred: %v, exiting", err)
//			}
//		}
//	case err = <-replyCh:
//		log.Debugln("callManager.outgoingCall: replyCh received", err)
//		if err != nil {
//			log.Printf("Error occurred: %v, exiting", err)
//		}
//	}
//}
func (p *grpcCallManager) CallAll(ctx context.Context) {
	log.Debugln("callManager.CallAll: enter")
	defer log.Debugln("callManager.CallAll: exit")
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		p.outgoingCall(ctx, address)
	}
}

//func startReceivings(ctx context.Context, stream pb.Intercom_DuplexCallClient, bytesCh chan float32, errCh chan error) {
//	log.Debugln("startReceiving: enter")
//	defer log.Debugln("startReceiving: exit")
//	for {
//		select {
//		case <-ctx.Done():
//			log.Printf("Context.Done: %v", ctx.Err())
//			errCh <- ctx.Err()
//			return
//
//		default:
//			bytes, err := stream.Recv()
//			log.Debugln("startReceiving: stream.Recv'd")
//			if err == io.EOF {
//				log.Println("Received end of data from incoming connection")
//				// errCh <- nil
//				return
//			}
//			if err != nil {
//				errCh <- err
//				return
//			}
//			// bytesCh <- bytes.Data
//			for _, b := range bytes.Data {
//				log.Debugln("startReceiving: sending to bytesCh")
//				bytesCh <- b
//				log.Debugln("startReceiving: sent to bytesCh")
//			}
//		}
//	}
//}
//
//// func Write(p []byte) (n int, err error)
//
//func startStreaming(ctx context.Context, stream pb.Intercom_DuplexCallClient, errCh chan error) {
//	log.Debugln("startStreaming: enter")
//	defer log.Debugln("startStreaming: exit")
//	c, err := pulse.NewClient()
//	if err != nil {
//		errCh <- err
//		return
//	}
//	defer log.Debugln("startStreaming: closed pulse client")
//	defer c.Close()
//	defer log.Debugln("startStreaming: closing pulse client")
//	b := pulseCallWriter{
//		pulseClient: c,
//		callClient:  stream,
//	}
//	micStream, err := c.NewRecord(pulse.Float32Writer(b.Write))
//	if err != nil {
//		log.Println("Unable to create new pulse recorder", err)
//		errCh <- err
//	}
//	log.Debugln("startStreaming: starting mic")
//	micStream.Start()
//	defer log.Debugln("startStreaming: closed micStream")
//	defer micStream.Stop()
//	defer log.Debugln("startStreaming: closing micStream")
//	log.Debugln("startStreaming: waiting until context is done")
//	<-ctx.Done()
//	log.Printf("Context.Done: %v", ctx.Err())
//	errCh <- ctx.Err()
//}
//
//type pulseCallWriter struct {
//	pulseClient *pulse.Client
//	callClient  pb.Intercom_DuplexCallClient
//}
//
//func (b pulseCallWriter) Write(p []float32) (n int, err error) {
//	log.Debugln("pulseCallWriter.Write: entered")
//	defer log.Debugln("pulseCallWriter.Write: exit")
//	c := b.callClient
//	data := pb.AudioData{
//		Data: p,
//	}
//	err = c.Send(&data)
//	n = len(p)
//	return
//}

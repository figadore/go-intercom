package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	audioInCh := make(chan float32, 256) // 256 is arbitrary
	audioOutCh := make(chan float32, 256)
	defer close(audioInCh)
	defer close(audioOutCh)

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
	defer func() { _ = serverStream.CloseSend() }() // doesn't return errors, always nil

	speakerBuf := audioBuffer{
		audioCh: audioInCh,
		ctx:     ctx,
	}
	micBuf := audioBuffer{
		audioCh: audioOutCh,
		ctx:     ctx,
	}

	go startReceiving(ctx, audioInCh, errCh, serverStream.Recv)
	go startPlayback(ctx, speakerBuf, errCh)
	go startRecording(ctx, micBuf, errCh)
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

func startReceiving(ctx context.Context, speakerCh chan<- float32, errCh chan error, recvFn func() (*pb.AudioData, error)) {
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				errCh <- err
			}
			return
		default:
			break
		}
		in, err := recvFn()
		if err == io.EOF {
			return
		} else if err != nil {
			errCh <- err
		}
		for _, b := range in.Data {
			speakerCh <- b
		}
	}
}

func startSending(ctx context.Context, micCh <-chan float32, errCh chan error, sendFn func(*pb.AudioData) error) {
	// to end streaming, send io.EOF and return
	audioBytes := make([]float32, 256) // 256 is arbitrary
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				// TODO find out how to close connection from here... send nil?
				_ = sendFn(nil)
				errCh <- err
			}
			return
		default:
			break
		}
		for i := 0; i < 256; i++ {
			d, ok := <-micCh
			if !ok {
				_ = sendFn(nil)
				errCh <- errors.New("startSending: mic audio channel appears closed")
				return
			}
			audioBytes[i] = d
		}
		data := pb.AudioData{
			Data: audioBytes,
		}
		err := sendFn(&data)
		if err != nil {
			errCh <- err
			return
		}
	}
}

type audioBuffer struct {
	audioCh chan float32
	ctx     context.Context
}

func (a audioBuffer) Read(buf []float32) (n int, err error) {
	for n = range buf {
		select {
		case <-a.ctx.Done():
			// return n, io.EOF
			return n, pulse.EndOfData
		case d, ok := <-a.audioCh:
			if !ok {
				return n, errors.New("speakerBuffer: audio channel appears closed")
			}
			buf[n] = d
		}
	}
	return
}

func (a audioBuffer) Write(buf []float32) (n int, err error) {
	for _, data := range buf {
		a.audioCh <- data
	}
	return n, nil
}

func startPlayback(ctx context.Context, speakerBuf audioBuffer, errCh chan error) {
	c, err := pulse.NewClient()
	if err != nil {
		errCh <- err
		return
	}
	defer c.Close()
	speakerStream, err := c.NewPlayback(pulse.Float32Reader(speakerBuf.Read), pulse.PlaybackLatency(.1))
	if err != nil {
		errCh <- err
		return
	}
	defer speakerStream.Close()
	speakerStream.Start()
	// Stream to speaker until context is cancelled
	<-ctx.Done()
	errCh <- ctx.Err()
	if err != nil {
		errCh <- err
		// to allow Drain() to return, send pulse.EndOfData from reader
		// this *should* happen when the context is cancelled for any reason (see speakerBuf.Read)
		log.Debugln("Draining speaker stream")
		speakerStream.Drain()
		log.Debugln("Drained speaker stream")
		return
	}
	log.Println("Underflow:", speakerStream.Underflow())
	if speakerStream.Error() != nil {
		err = speakerStream.Error()
		errCh <- err
		return
	}
}

func startRecording(ctx context.Context, micBuf audioBuffer, errCh chan error) {
	c, err := pulse.NewClient()
	if err != nil {
		errCh <- err
		return
	}
	defer c.Close()
	micStream, err := c.NewRecord(pulse.Float32Writer(micBuf.Write))
	if err != nil {
		errCh <- err
		return

	}
	defer micStream.Close()
	micStream.Start() // async
	<-ctx.Done()
	err = ctx.Err()
	if err != nil {
		log.Debugln("Stopping speaker stream")
		micStream.Stop()
		errCh <- err
		return
	}
}

func (p *grpcCallManager) CallAll(ctx context.Context) {
	log.Debugln("callManager.CallAll: enter")
	defer log.Debugln("callManager.CallAll: exit")
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		p.outgoingCall(ctx, address)
	}
}

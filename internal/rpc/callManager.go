package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/jar-o/limlog"
	"github.com/jfreymuth/pulse"
	"google.golang.org/grpc"

	//"github.com/figadore/go-intercom/internal/log"
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
	log.Println("outgoingCall: Start client side DuplexCall")
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
	log.Println("Debug: dialed")
	defer conn.Close()
	client := pb.NewIntercomClient(conn)
	serverStream, err := client.DuplexCall(ctx)
	if err != nil {
		log.Printf("placeCall: error creating duplex call client: %v", err)
		return
	}
	defer func() { _ = serverStream.CloseSend() }() // doesn't return errors, always nil

	ll := limlog.NewLimlog()
	ll.SetLimiter("limiter1", 1, 1*time.Second, 6)
	speakerBuf := audioBuffer{
		audioCh: audioInCh,
		ctx:     ctx,
		ll:      ll,
	}
	micBuf := audioBuffer{
		audioCh: audioOutCh,
		ctx:     ctx,
		ll:      ll,
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
	log.Println("startReceiving: enter")
	defer log.Println("startReceiving: exit")
	ll := limlog.NewLimlog()
	ll.SetLimiter("limiter1", 1, 1*time.Second, 6)
	// log.SetPrefix("startReceiving: ")
	// log.SetFlags(log.Ldate | log.Lmicroseconds)
	for {
		select {
		case <-ctx.Done():
			log.Println("startReceiving: context done")
			if err := ctx.Err(); err != nil {
				log.Println("startReceiving: context error", ctx.Err())
				errCh <- err
			}
			return
		default:
			break
		}
		in, err := recvFn()
		if err == io.EOF {
			log.Println("startReceiving: received io.EOF")
			return
		} else if err != nil {
			log.Println("startReceiving: error receiving", err)
			errCh <- err
			return
		}
		for _, b := range in.Data {
			ll.DebugL("sending to speakerCh")
			speakerCh <- b
			ll.DebugL("speakerCh sent")
		}
	}
}

func startSending(ctx context.Context, micCh <-chan float32, errCh chan error, sendFn func(*pb.AudioData) error) {
	log.Println("startSending: enter")
	defer log.Println("startSending: exit")
	// to end streaming, send io.EOF and return
	audioBytes := make([]float32, 256) // 256 is arbitrary
	ll := limlog.NewLimlog()
	ll.SetLimiter("limiter1", 1, 1*time.Second, 6)
	for {
		select {
		case <-ctx.Done():
			log.Println("startSending: context done")
			if err := ctx.Err(); err != nil {
				// TODO find out how to close connection from here... send nil?
				//log.Println("startSending: Warning, sending nil")
				//_ = sendFn(nil)
				log.Println("startSending: context error", err)
				errCh <- err
			}
			return
		default:
			break
		}
		for i := 0; i < 256; i++ {
			d, ok := <-micCh
			if !ok {
				msg := "startSending: mic audio channel appears closed"
				log.Println(msg)
				//log.Println("startSending: Warning, sending nil 2")
				//_ = sendFn(nil)
				errCh <- errors.New(msg)
				return
			}
			audioBytes[i] = d
		}
		data := pb.AudioData{
			Data: audioBytes,
		}
		ll.DebugL("sending data to grpc")
		err := sendFn(&data)
		ll.DebugL("grpc data sent")
		if err != nil {
			log.Println("startSending: error grpc sending", err)
			errCh <- err
			return
		}
	}
}

type audioBuffer struct {
	audioCh chan float32
	ctx     context.Context
	ll      *limlog.Limlog
}

func (a audioBuffer) Read(buf []float32) (n int, err error) {
	for n = range buf {
		select {
		case <-a.ctx.Done():
			// return n, io.EOF
			return n, pulse.EndOfData
		case d, ok := <-a.audioCh:
			a.ll.DebugL("audioBuffer.Read received from audioCh")
			if !ok {
				return n, errors.New("speakerBuffer: audio channel appears closed")
			}
			buf[n] = d
		}
	}
	return
}

func (a audioBuffer) Write(buf []float32) (n int, err error) {
	a.ll.DebugL("audioBuffer.Write with this many float32s:", len(buf))
	for i, data := range buf {
		a.ll.DebugL("audioBuffer.Write sending to audioCh", i)
		a.audioCh <- data
		a.ll.DebugL("audioBuffer.Write sent")
	}
	return n, nil
}

func startPlayback(ctx context.Context, speakerBuf audioBuffer, errCh chan error) {
	log.Println("startPlayback: enter")
	defer log.Println("startPlayback: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startPlayback: error creating pulse client", err)
		errCh <- err
		return
	}
	defer c.Close()
	speakerStream, err := c.NewPlayback(pulse.Float32Reader(speakerBuf.Read), pulse.PlaybackLatency(.1))
	if err != nil {
		log.Println("startPlayback: error creating speaker stream", err)
		errCh <- err
		return
	}
	defer speakerStream.Close()
	speakerStream.Start()
	// Stream to speaker until context is cancelled
	<-ctx.Done()
	log.Println("startPlayback: context done")
	errCh <- ctx.Err()
	if err != nil {
		log.Println("startPlayback: context error", err)
		errCh <- err
		// to allow Drain() to return, send pulse.EndOfData from reader
		// this *should* happen when the context is cancelled for any reason (see speakerBuf.Read)
		log.Println("Debug: Draining speaker stream")
		speakerStream.Drain()
		log.Println("Debug: Drained speaker stream")
		return
	}
	log.Println("Underflow:", speakerStream.Underflow())
	if speakerStream.Error() != nil {
		err = speakerStream.Error()
		log.Println("startPlayback: speakerStream error", err)
		errCh <- err
		return
	}
}

func startRecording(ctx context.Context, micBuf audioBuffer, errCh chan error) {
	log.Println("startRecording: enter")
	defer log.Println("startRecording: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startRecording: error creating pulse client", err)
		errCh <- err
		return
	}
	defer c.Close()
	micStream, err := c.NewRecord(pulse.Float32Writer(micBuf.Write))
	if err != nil {
		log.Println("startRecording: error creating new recorder", err)
		errCh <- err
		return

	}
	defer micStream.Close()
	micStream.Start() // async
	<-ctx.Done()
	log.Println("startRecording: context done")
	err = ctx.Err()
	if err != nil {
		log.Println("startRecording: context error", err)
		micStream.Stop()
		errCh <- err
		return
	}
}

func (p *grpcCallManager) CallAll(ctx context.Context) {
	log.Println("Debug: callManager.CallAll: enter")
	defer log.Println("Debug: callManager.CallAll: exit")
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		// TODO do this with a go routine if there are more than one clients
		p.outgoingCall(ctx, address)
	}
}

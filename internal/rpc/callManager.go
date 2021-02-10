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

const (
	// fragmentSize = 44100 / 20
	fragmentSize = 1600
	sampleRate   = 8000
)

type grpcCallManager struct {
	callList []call.Call
	station  *station.Station
}

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

type audioBuffer struct {
	audioCh  chan []float32
	buffered []float32
	ctx      context.Context
}

func (p *grpcCallManager) outgoingCall(parentContext context.Context, address string) {
	log.Println("outgoingCall: Start client side DuplexCall")
	select {
	case <-parentContext.Done():
		msg := fmt.Sprintf("outgoingCall: Error: parent context cancelled: %v", parentContext.Err())
		log.Println(msg)
		return
	default:
		break
	}
	ctx, cancel := context.WithCancel(context.WithValue(parentContext, call.ContextKey("address"), address))
	defer cancel()
	// TODO call.New()?
	currentCall := call.Call{
		To:     address,
		From:   "self",
		Cancel: cancel,
		Status: call.CallStatusPending,
	}
	p.callList = append(p.callList, currentCall)
	errCh := make(chan error)
	audioInCh := make(chan []float32)
	audioOutCh := make(chan []float32)
	// Things seem to break when these are uncommented, still not sure why
	// defer close(audioInCh)
	// defer close(audioOutCh)

	// Initiate a grpc connection with the server
	fullAddress := fmt.Sprintf("192.168.0.%s%s", address, port)
	log.Println("dialing", fullAddress)
	conn, err := grpc.Dial(fullAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		msg := fmt.Sprintf("Warning: Unable to dial %v: %v", fullAddress, err)
		log.Println(msg)
		return
	}
	log.Debugln("Debug: dialed")
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
	go startPlayback(ctx, &speakerBuf, errCh)
	go startRecording(ctx, &micBuf, errCh)
	go startSending(ctx, audioOutCh, errCh, serverStream.Send)
	select {
	case <-ctx.Done():
		log.Printf("placeCall: context.Done: %v", ctx.Err())
		return
	case <-parentContext.Done():
		log.Printf("warning: placeCall: parentContext.Done. didn't know this was possible: %v", ctx.Err())
		return
	case err := <-errCh:
		log.Printf("placeCall: errCh: %v", err)
		return
	}
}

// Infinite loop to receive from the gRPC stream and send it to the speaker
func startReceiving(ctx context.Context, speakerCh chan<- []float32, errCh chan error, recvFn func() (*pb.AudioData, error)) {
	log.Println("startReceiving: enter")
	defer log.Println("startReceiving: exit")
	// log.SetPrefix("startReceiving: ")
	// log.SetFlags(log.Ldate | log.Lmicroseconds)
	for {
		select {
		case <-ctx.Done():
			log.Println("startReceiving: context done")
			if err := ctx.Err(); err != nil {
				log.Println("startReceiving: context error", ctx.Err())
				errCh <- err
				log.Println("startReceiving: sent context error", ctx.Err())
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
		data := make([]float32, len(in.Data))
		copy(data, in.Data)
		speakerCh <- data
	}
}

// Infinite loop to receive from the mic and stream it to the gRPC server
func startSending(ctx context.Context, micCh <-chan []float32, errCh chan error, sendFn func(*pb.AudioData) error) {
	log.Println("startSending: enter")
	defer log.Println("startSending: exit")
	// when streaming ends, client receives io.EOF
	// not sure how to initiate this on the server side though
	var audioBytes []float32
	var ok bool
	var data pb.AudioData
	for {
		select {
		case <-ctx.Done():
			log.Println("startSending: context done")
			if err := ctx.Err(); err != nil {
				// TODO find out how to close connection from here... send nil?
				log.Println("startSending: context error", err)
				errCh <- err
			}
			return
		default:
			break
		}
		audioBytes, ok = <-micCh
		data = pb.AudioData{
			Data: audioBytes,
		}
		err := sendFn(&data)
		if err != nil {
			log.Println("startSending: error grpc sending", err)
			errCh <- err
			return
		}
	}
}

// Read receives data from the audio data channel and sends it to the speaker
func (a *audioBuffer) Read(buf []float32) (n int, err error) {
	select {
	case <-a.ctx.Done():
		// close(a.audioCh)
		return n, pulse.EndOfData
	default:
		break
	}
	if len(a.buffered) == 0 {
		// blocking
		a.fill()
	}
	copy(buf, a.buffered)
	if len(a.buffered) >= len(buf) {
		n = len(buf)
	} else {
		n = len(a.buffered)
	}
	a.buffered = a.buffered[n:]
	return
}

// fill receives from the audio channel and places it in a buffer
func (a *audioBuffer) fill() {
	// TODO check for closed channel
	data := <-a.audioCh
	a.buffered = append(a.buffered, data...)
}

// Write sends the data from the microphone buffer and sends it to the audio data channel
func (a *audioBuffer) Write(buf []float32) (n int, err error) {
	select {
	case <-a.ctx.Done():
		// close(a.audioCh)
		return n, pulse.EndOfData
	default:
		break
	}
	data := make([]float32, len(buf))
	copy(data, buf)
	a.audioCh <- data
	n = len(buf)
	return n, nil
}

// startPlayback receives data from a channel and plays through the speaker
func startPlayback(ctx context.Context, speakerBuf *audioBuffer, errCh chan error) {
	log.Println("startPlayback: enter")
	defer log.Println("startPlayback: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startPlayback: error creating pulse client", err)
		errCh <- err
		return
	}
	defer c.Close()
	speakerStream, err := c.NewPlayback(pulse.Float32Reader(speakerBuf.Read), pulse.PlaybackSampleRate(sampleRate), pulse.PlaybackBufferSize(fragmentSize))
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
		// this should happen when the context is cancelled for any reason (see speakerBuf.Read)
		log.Debugln("Debug: Draining speaker stream")
		speakerStream.Drain()
		log.Debugln("Debug: Drained speaker stream")
		return
	}
	log.Println("Underflow:", speakerStream.Underflow())
	if speakerStream.Error() != nil {
		err = speakerStream.Error()
		log.Println("startPlayback: speakerStream error", err)
		errCh <- err
		log.Println("startPlayback: sent speakerStream error", err)
		return
	}
}

// startRecording gets data from the microphone and sends it to the audio channel
func startRecording(ctx context.Context, micBuf *audioBuffer, errCh chan error) {
	log.Println("startRecording: enter")
	defer log.Println("startRecording: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startRecording: error creating pulse client", err)
		errCh <- err
		return
	}
	defer c.Close()
	micStream, err := c.NewRecord(pulse.Float32Writer(micBuf.Write), pulse.RecordSampleRate(sampleRate), pulse.RecordBufferFragmentSize(fragmentSize))
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
		log.Println("startRecording: sending error", err)
		errCh <- err
		log.Println("startRecording: sent error", err)
		return
	}
}

// CallAll calls every intercom station it knows about
func (p *grpcCallManager) CallAll(ctx context.Context) {
	log.Debugln("Debug: callManager.CallAll: enter")
	defer log.Debugln("Debug: callManager.CallAll: exit")
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		// TODO do this with a go routine if there are more than one clients?
		p.outgoingCall(ctx, address)
	}
}

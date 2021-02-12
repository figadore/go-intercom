package rpc

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rs/xid"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/rpc/pb"
	"github.com/figadore/go-intercom/internal/station"
	"github.com/figadore/go-intercom/pkg/call"
)

type grpcCallManager struct {
	call.GenericManager
	station *station.Station
}

func (callManager *grpcCallManager) Hangup() {
	for _, c := range callManager.CallList {
		c.Hangup()
		callManager.removeCall(c)
	}
	callManager.station.UpdateStatus()
}

func NewCallManager(intercom *station.Station) call.Manager {
	m := &grpcCallManager{
		station: intercom,
	}
	m.CallList = make(map[xid.ID]call.Call)
	return m
}

func (callManager *grpcCallManager) SetStation(s *station.Station) {
	callManager.station = s
}

func (callManager *grpcCallManager) addCall(c call.Call) {
	_ = callManager.station.Status.Set(station.StatusCallConnected)
	callManager.CallList[c.Id] = c
}

func (callManager *grpcCallManager) removeCall(c call.Call) {
	delete(callManager.CallList, c.Id)
	if len(callManager.CallList) == 0 {
		_ = callManager.station.Status.Clear(station.StatusCallConnected)
	}
}

type streamer interface {
	Send(*pb.AudioData) error
	Recv() (*pb.AudioData, error)
}

func (callManager *grpcCallManager) duplexCall(parentContext context.Context, from string, to string, stream streamer) error {
	callId := xid.New()
	ctx, cancel := context.WithCancel(context.WithValue(parentContext, call.ContextKey("id"), callId))
	c := call.Call{
		Id:     callId,
		To:     to,
		From:   from,
		Cancel: cancel,
		Status: call.StatusActive,
	}
	defer cancel()
	log.Println("Starting call with id:", ctx.Value(call.ContextKey("id")))
	errCh := make(chan error)
	intercom := callManager.station
	callManager.addCall(c)
	defer callManager.removeCall(c)
	go callManager.startReceiving(ctx, errCh, stream.Recv)
	go intercom.StartPlayback(ctx, errCh)
	go intercom.StartRecording(ctx, errCh)
	go callManager.startSending(ctx, errCh, stream.Send)
	log.Debugln("DuplexCall: go routines started")
	select {
	case <-ctx.Done():
		log.Printf("Server.DuplexCall: context.Done: %v", ctx.Err())
		return ctx.Err()
	case err := <-errCh:
		log.Printf("Server.DuplexCall: errCh: %v", err)
		return err
	}
}

// CallAll calls every intercom station it knows about
// ctx is station context/main context from cmd
func (callManager *grpcCallManager) CallAll(mainContext context.Context) {
	log.Debugln("Debug: callManager.CallAll: enter")
	defer log.Debugln("Debug: callManager.CallAll: exit")
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		go callManager.outgoingCall(mainContext, address)
	}
}

func (callManager *grpcCallManager) outgoingCall(mainContext context.Context, address string) {
	log.Println("outgoingCall: Start client side DuplexCall")
	select {
	case <-mainContext.Done():
		msg := fmt.Sprintf("outgoingCall: Error: parent context cancelled: %v", mainContext.Err())
		log.Println(msg)
		return
	default:
		break
	}

	// Initiate a grpc connection with the server
	fullAddress := fmt.Sprintf("192.168.0.%s%s", address, port)
	to := fullAddress
	from := "self"
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
	serverStream, err := client.DuplexCall(callManager.station.Context)
	if err != nil {
		log.Printf("placeCall: error creating duplex call client: %v", err)
		return
	}
	defer func() { _ = serverStream.CloseSend() }() // doesn't return errors, always nil
	err = callManager.duplexCall(mainContext, to, from, serverStream)
	log.Println("Client-side duplex call ended with:", err)
}

// Infinite loop to receive from the gRPC stream and send it to the speaker
func (callManager *grpcCallManager) startReceiving(ctx context.Context, errCh chan error, recvFn func() (*pb.AudioData, error)) {
	log.Println("startReceiving: enter")
	defer log.Println("startReceiving: exit")
	intercom := callManager.station
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
		intercom.SendSpeakerAudio(data)
	}
}

// Infinite loop to receive from the mic and stream it to the gRPC server
func (callManager *grpcCallManager) startSending(ctx context.Context, errCh chan error, sendFn func(*pb.AudioData) error) {
	log.Println("startSending: enter")
	defer log.Println("startSending: exit")
	intercom := callManager.station
	// when streaming ends, client receives io.EOF
	// not sure how to initiate this on the server side though
	var audioBytes []float32
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
		audioBytes = intercom.ReceiveMicAudio()
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

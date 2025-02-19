package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/rpc/pb"
	"github.com/figadore/go-intercom/internal/station"
	"github.com/figadore/go-intercom/pkg/call"
)

type grpcCallManager struct {
	call.GenericManager
	station  *station.Station
	acceptCh chan bool
}

func (callManager *grpcCallManager) HangupAll() {
	for _, c := range callManager.CallList {
		c.Hangup()
		callManager.removeCall(c)
	}
	callManager.station.UpdateStatus()
}

func NewCallManager(intercom *station.Station) call.Manager {
	m := &grpcCallManager{
		station:  intercom,
		acceptCh: make(chan bool),
	}
	m.CallList = make(map[call.CallId]*call.Call)
	return m
}

func (callManager *grpcCallManager) AcceptCh() chan bool {
	return callManager.acceptCh
}

func (callManager *grpcCallManager) SetStation(s *station.Station) {
	callManager.station = s
}

func (callManager *grpcCallManager) addCall(c *call.Call) {
	callManager.CallList[c.Id] = c
}

func (callManager *grpcCallManager) AcceptCall() {
}

func (callManager *grpcCallManager) RejectCall() {
}

func (callManager *grpcCallManager) removeCall(c *call.Call) {
	delete(callManager.CallList, c.Id)
	if len(callManager.CallList) == 0 {
		_ = callManager.station.Status.Clear(station.StatusCallConnected)
	}
}

type streamer interface {
	Send(*pb.AudioData) error
	Recv() (*pb.AudioData, error)
}

// A cancel function is passed in here so that the grpc stream's context can be cancelled
func (callManager *grpcCallManager) duplexCall(parentContext context.Context, from string, to string, stream streamer, cancel func()) error {
	defer log.Println("callManager.duplexCall: Exiting, no more error receivable")
	callId := call.NewCallId()
	log.Debugf("Created new callId: %v", callId)
	callContext := context.WithValue(parentContext, call.ContextKey("id"), callId)
	c := call.New(callId, to, from, cancel)
	defer log.Debugln("duplexCall: call.Hangup() complete")
	defer c.Hangup()
	defer log.Debugln("duplexCall: call.Hangup() is next, should cancel goroutines' contexts")
	intercom := callManager.station
	defer intercom.UpdateStatus()
	callManager.addCall(c)
	defer callManager.removeCall(c) // TODO remove this when c.Hangup() removes the call
	log.Printf("Starting call with id %v:", callContext.Value(call.ContextKey("id")))
	errCh := make(chan error)
	var wg sync.WaitGroup
	wg.Add(4)
	connected := initializeConnection(callContext, stream.Send, stream.Recv, errCh)
	if !connected {
		msg := "Call did not initialize"
		log.Println(msg)
		return errors.New(msg)
	}
	c.Status = call.StatusActive
	callManager.station.Status.Set(station.StatusCallConnected)
	callManager.station.Status.Clear(station.StatusOutgoingCall)
	callManager.station.Status.Clear(station.StatusIncomingCall)
	go callManager.startSending(callContext, &wg, errCh, stream.Send)
	go callManager.startReceiving(callContext, &wg, errCh, stream.Recv)
	go intercom.StartRecording(callContext, &wg, errCh)
	go intercom.StartPlayback(callContext, &wg, errCh)
	log.Debugln("DuplexCall: go routines started")
	select {
	case <-callContext.Done():
		log.Printf("duplexCall: context.Done: %v", callContext.Err())
		cancel()
		wg.Wait()
		return callContext.Err()
	case err := <-errCh:
		log.Printf("duplexCall: Received error on errCh: %v", err)
		cancel()
		log.Printf("duplexCall: waiting on waitgroup")
		wg.Wait()
		log.Printf("duplexCall: finished waiting on waitgroup")
		return err
	}
}

// Send and receive the first packets of data. These will be empty slices
func initializeConnection(ctx context.Context, sendFn func(*pb.AudioData) error, recvFn func() (*pb.AudioData, error), errCh chan error) bool {
	// Initial send
	data := pb.AudioData{
		Data: make([]float32, 0),
	}
	err := sendFn(&data)
	if err != nil {
		log.Println("startSending.initialSend: error grpc sending", err)
		sendWithTimeout(err, errCh)
		log.Println("startSending.initialSend: sent error grpc sending", err)
		return false
	}
	log.Println("startSending.initialSend: sent first packet", err)

	// Initial receive
	_, err = recvFn()
	if err == io.EOF {
		log.Println("startReceiving.initialReceive: received io.EOF")
		return false
	} else if err != nil {
		log.Println("startReceiving.initialReceive: error receiving", err)
		select {
		case <-ctx.Done():
			log.Println("startReceiving.initialReceive: context done4")
		case errCh <- err:
			log.Println("startReceiving.initialReceive: sent error receiving", err)
		case <-time.After(5 * time.Second):
			log.Println("WARN: startReceiving.initialReceive: timeout sending error", err)
		}
		return false
	}
	log.Println("startReceiving.initialReceive: received first packet")
	return true
}

// CallAll calls every intercom station it knows about
// ctx is station context/main context from cmd
func (callManager *grpcCallManager) CallAll() {
	log.Debugln("Debug: callManager.CallAll: enter")
	defer log.Debugln("Debug: callManager.CallAll: exit")
	intercoms := os.Args[1:]
	for _, address := range intercoms {
		go callManager.outgoingCall(address)
		// callManager.outgoingCall(mainContext, address)
	}
}

func (callManager *grpcCallManager) outgoingCall(address string) {
	log.Println("outgoingCall: Start client side DuplexCall")

	// Initiate a grpc connection with the server
	fullAddress := fmt.Sprintf("192.168.0.%s%s", address, port)
	to := fullAddress
	from := "self"
	log.Println("outgoingCall: dialing", fullAddress)
	_ = callManager.station.Status.Set(station.StatusOutgoingCall)
	// TODO send separate "call request" grpc call to ring other end and wait if auto-answer not enabled? or set status once first successful send/receive happens?
	conn, err := grpc.Dial(fullAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		msg := fmt.Sprintf("Warning: Unable to dial %v: %v", fullAddress, err)
		log.Println(msg)
		return
	}
	log.Debugln("outgoingCall: dialed")
	defer log.Debugln("outgoingCall: conn.Closed")
	defer conn.Close()
	defer log.Debugln("outgoingCall: conn.Closing")
	client := pb.NewIntercomClient(conn)
	grpcCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverStream, err := client.DuplexCall(grpcCtx)
	if err != nil {
		log.Printf("outgoingCall: error creating duplex call client: %v", err)
		return
	}
	defer func() {
		// doesn't return errors, always nil
		log.Println("outgoingCall: CloseSend()")
		_ = serverStream.CloseSend()
		log.Println("outgoingCall: CloseSend() complete")
	}()
	err = callManager.duplexCall(grpcCtx, to, from, serverStream, cancel)
	log.Println("outgoingCall: client-side duplex call ended with:", err)
}

func sendWithTimeout(err error, errCh chan error) {
	select {
	case errCh <- err:
	case <-time.After(5 * time.Second):
		log.Println("WARN: timeout sending error", err)
	}
}

// Infinite loop to receive from the gRPC stream and send it to the speaker
func (callManager *grpcCallManager) startReceiving(ctx context.Context, wg *sync.WaitGroup, errCh chan error, recvFn func() (*pb.AudioData, error)) {
	log.Println("startReceiving: enter")
	defer log.Println("startReceiving: exit")
	defer wg.Done()
	intercom := callManager.station
	// log.SetPrefix("startReceiving: ")
	// log.SetFlags(log.Ldate | log.Lmicroseconds)
	for {
		select {
		case <-ctx.Done():
			log.Println("startReceiving: context done")
			if err := ctx.Err(); err != nil {
				log.Println("startReceiving: context error", ctx.Err())
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
			select {
			case <-ctx.Done():
				log.Println("startReceiving: context done2")
			case errCh <- err:
				log.Println("startReceiving: sent error receiving", err)
			case <-time.After(5 * time.Second):
				log.Println("WARN: startReceiving: timeout sending error", err)
			}
			return
		}
		data := make([]float32, len(in.Data))
		copy(data, in.Data)
		select {
		case <-ctx.Done():
			log.Println("startReceiving: context done3: ", ctx.Err())
			return
		case intercom.SpeakerAudioCh() <- data:
		}
	}
}

//func sendErr(err, errCh) {
//			select {
//			case errCh <- err:
//				log.Println("startReceiving: sent recv error", ctx.Err())
//			case <-time.After(5 * time.Second):
//				log.Println("startReceiving: timeout sending recv error", ctx.Err())
//			}
//}

// Infinite loop to receive from the mic and stream it to the gRPC server
func (callManager *grpcCallManager) startSending(ctx context.Context, wg *sync.WaitGroup, errCh chan error, sendFn func(*pb.AudioData) error) {
	log.Println("startSending: enter")
	defer log.Println("startSending: exit")
	defer wg.Done()
	intercom := callManager.station
	// when streaming ends, client receives io.EOF
	// not sure how to initiate this on the server side though
	var audioBytes []float32
	var data pb.AudioData
	for {
		select {
		case audioBytes = <-intercom.MicAudioCh():
			data = pb.AudioData{
				Data: audioBytes,
			}
		case <-time.After(5 * time.Second):
			log.Println("WARN: timeout receiving from mic audio channel")
			return
		case <-ctx.Done():
			log.Println("startSending: context done")
			_ = sendFn(nil)
			return
			// default:
			//	break
		}
		err := sendFn(&data)
		if err != nil {
			log.Println("startSending: error grpc sending", err)
			sendWithTimeout(err, errCh)
			log.Println("startSending: sent error grpc sending", err)
			return
		}
	}
}

package webrtc

import (
	"context"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/pion/webrtc/v3"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/station"
	"github.com/figadore/go-intercom/pkg/call"
)

type webrtcCallManager struct {
	call.GenericManager
	station  *station.Station
	acceptCh chan bool
}

func NewCallManager(intercom *station.Station) call.Manager {
	m := &webrtcCallManager{
		station:  intercom,
		acceptCh: make(chan bool),
	}
	m.CallList = make(map[call.CallId]*call.Call)
	return m
}

// TODO move these methods to generic call manager
func (callManager *webrtcCallManager) AcceptCh() chan bool {
	return callManager.acceptCh
}

func (callManager *webrtcCallManager) HangupAll() {
	for _, c := range callManager.CallList {
		c.Hangup()
		callManager.removeCall(c)
	}
	callManager.station.UpdateStatus()
}

func (callManager *webrtcCallManager) removeCall(c *call.Call) {
	delete(callManager.CallList, c.Id)
	if len(callManager.CallList) == 0 {
		_ = callManager.station.Status.Clear(station.StatusCallConnected)
	}
}

func (callManager *webrtcCallManager) addCall(c *call.Call) {
	callManager.CallList[c.Id] = c
}

// end move these methods to generic call manager

func (callManager *webrtcCallManager) AcceptCall() {
}

func (callManager *webrtcCallManager) RejectCall() {
}

// CallAll calls every intercom station it knows about
// ctx is station context/main context from cmd
func (callManager *webrtcCallManager) CallAll() {
	log.Debugln("Debug: callManager.CallAll: enter")
	defer log.Debugln("Debug: callManager.CallAll: exit")
	//intercoms := os.Args[1:]
	//for _, address := range intercoms {
	//	go callManager.outgoingCall(address)
	//	// callManager.outgoingCall(mainContext, address)
	//}
}

func (callManager *webrtcCallManager) duplexCall(parentContext context.Context, cancel func(), dataChannel *webrtc.DataChannel) error {
	defer log.Println("callManager.duplexCall: Exiting, no more error receivable")
	callId := call.NewCallId()
	log.Debugf("Created new callId: %v", callId)
	callContext := context.WithValue(parentContext, call.ContextKey("id"), callId)
	c := call.New(callId, "unknown", "unknown", cancel)
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
	c.Status = call.StatusActive
	intercom.Status.Set(station.StatusCallConnected)
	intercom.Status.Clear(station.StatusOutgoingCall)
	intercom.Status.Clear(station.StatusIncomingCall)
	go callManager.startSending(callContext, &wg, errCh, dataChannel)
	go callManager.startReceiving(callContext, &wg, errCh, dataChannel)
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

// Infinite loop to receive from the mic and stream it through webrtc
func (callManager *webrtcCallManager) startSending(ctx context.Context, wg *sync.WaitGroup, errCh chan error, dataChannel *webrtc.DataChannel) {
	dataChannel.OnOpen(func() {
		log.Println("startSending: enter")
		defer log.Println("startSending: exit")
		defer wg.Done()
		intercom := callManager.station
		var audioBytes []float32
		for {
			select {
			case audioBytes = <-intercom.MicAudioCh():
			case <-time.After(5 * time.Second):
				log.Println("WARN: timeout receiving from mic audio channel")
				return
			case <-ctx.Done():
				log.Println("startSending: context done")
				_ = dataChannel.Send(nil)
				return
				// default:
				//	break
			}
			err := dataChannel.Send(byteSlice(audioBytes))
			if err != nil {
				log.Println("startSending: error grpc sending", err)
				sendWithTimeout(err, errCh)
				log.Println("startSending: sent error grpc sending", err)
				return
			}
		}
	})
}

func sendWithTimeout(err error, errCh chan error) {
	select {
	case errCh <- err:
	case <-time.After(5 * time.Second):
		log.Println("WARN: timeout sending error", err)
	}
}

func byteSlice(s []float32) []byte {
	h := *(*reflect.SliceHeader)(unsafe.Pointer(&s))
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: h.Data, Len: h.Len * 4, Cap: h.Len * 4}))
}

func float32Slice(s []byte) []float32 {
	h := *(*reflect.SliceHeader)(unsafe.Pointer(&s))
	return *(*[]float32)(unsafe.Pointer(&reflect.SliceHeader{Data: h.Data, Len: h.Len / 4, Cap: h.Len / 4}))
}

// Infinite loop to receive from the webrtc stream and send it to the speaker
func (callManager *webrtcCallManager) startReceiving(ctx context.Context, wg *sync.WaitGroup, errCh chan error, dataChannel *webrtc.DataChannel) {
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		intercom := callManager.station

		audioData := float32Slice(msg.Data)
		select {
		case <-ctx.Done():
			log.Println("startReceiving: context done3: ", ctx.Err())
			return
		case intercom.SpeakerAudioCh() <- audioData:
		}
	})
}

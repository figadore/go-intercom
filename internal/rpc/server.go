package rpc

import (
	"context"
	"net"
	"time"

	"github.com/jar-o/limlog"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/rpc/pb"
	"github.com/figadore/go-intercom/pkg/call"
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
	address := "unknown"
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), call.ContextKey("address"), address))
	currentCall := call.Call{
		To:     "self",
		From:   address,
		Cancel: cancel,
		Status: call.CallStatusActive,
	}
	callManager := NewCallManager()
	callManager.callList = append(callManager.callList, currentCall)
	log.Debugln("DuplexCall: context created")
	defer cancel()
	errCh := make(chan error)
	audioInCh := make(chan float32, 256)
	audioOutCh := make(chan float32, 256)
	log.Debugln("DuplexCall: channels created")
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
	log.Debugln("DuplexCall: buffers created")
	go startReceiving(ctx, audioInCh, errCh, clientStream.Recv)
	go startPlayback(ctx, speakerBuf, errCh)
	go startRecording(ctx, micBuf, errCh)
	go startSending(ctx, audioOutCh, errCh, clientStream.Send)
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

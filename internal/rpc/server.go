package rpc

import (
	"context"
	"net"

	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/rpc/pb"
	"github.com/figadore/go-intercom/internal/station"
)

func NewServer(intercom *station.Station) *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterIntercomServer(s, &Server{
		station: intercom,
		ctx:     intercom.Context,
	})
	return s
}

func Serve(s *grpc.Server, errCh chan error) {
	log.Debugf("Start net.Listen: %v", port)
	defer log.Debugf("Serve: Finished net.Listen: %v", port)
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Printf("Serve: failed to listen: %v", err)
		errCh <- err
	}
	if err := s.Serve(lis); err != nil {
		log.Printf("Serve: failed to serve: %v", err)
		errCh <- err
	}
}

type Server struct {
	pb.UnimplementedIntercomServer
	station *station.Station
	ctx     context.Context
}

func (s *Server) DuplexCall(clientStream pb.Intercom_DuplexCallServer) error {
	log.Println("Start server side DuplexCall")
	callManager := s.station.CallManager.(*grpcCallManager)
	err := callManager.duplexCall(s.ctx, clientStream)
	log.Println("Server-side duplex call ended with:", err)
	return err
}

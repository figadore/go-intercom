package rpc

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/rpc/pb"
	"github.com/figadore/go-intercom/internal/station"
)

func NewServer(intercom *station.Station) *grpc.Server {
	s := grpc.NewServer()
	pb.RegisterIntercomServer(s, &Server{
		station: intercom,
		// ctx:     intercom.Context,
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
}

// DuplexCall is run whenever the server receives an incoming call
// Return nil to end stream. client receives io.EOF
func (s *Server) DuplexCall(clientStream pb.Intercom_DuplexCallServer) error {
	s.station.Status.Set(station.StatusIncomingCall)
	// If calls already active, accept call
	if s.station.Status.Has(station.StatusCallConnected) {
		log.Println("One or more calls already active, auto-answering")
	} else if s.station.Status.Has(station.StatusDoNotDisturb) {
		// Wait for call call manager accept/reject-call function
		acceptCh := s.station.CallManager.AcceptCh()
		log.Println("Waiting 20 seconds for call to be accepted")
		select {
		case accept := <-acceptCh:
			if accept {
				log.Println("Call accepted")
			} else {
				log.Println("Call rejected")
				s.station.Status.Clear(station.StatusIncomingCall)
				return nil
			}
		case <-time.After(20 * time.Second):
			log.Println("Call rejected")
			s.station.Status.Clear(station.StatusIncomingCall)
			return nil
		}
	}
	log.Println("Server accepting call")
	// Update status appropriately
	s.station.Status.Clear(station.StatusIncomingCall)
	streamCtx := clientStream.Context()
	p, _ := peer.FromContext(streamCtx)
	addrPort := p.Addr.String()
	md, _ := metadata.FromIncomingContext(streamCtx)
	to := md[":authority"][0]
	from := addrPort

	log.Println("Start server side DuplexCall, receiving from ", addrPort)
	grpcCtx, cancel := context.WithCancel(streamCtx)
	// TODO figure out whether this cancel should be the one in the Call object
	defer cancel()
	callManager := s.station.CallManager.(*grpcCallManager)
	err := callManager.duplexCall(grpcCtx, to, from, clientStream, cancel)
	log.Println("Server-side duplex call ended with:", err)
	return err
}

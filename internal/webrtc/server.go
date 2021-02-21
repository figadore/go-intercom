package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/pion/webrtc/v3"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/webrtc/pb"
)

const (
	port = ":20000"
)

func NewServer(host string) (*grpc.Server, *Server) {
	s := grpc.NewServer()
	addr := fmt.Sprintf("192.168.0.%v%v", host, port)
	peerConnection, pendingCandidates := getPeerConnection(addr)
	server := &Server{
		pendingIceCandidates: pendingCandidates,
		peerAddr:             addr,
		peerConnection:       peerConnection,
	}
	pb.RegisterWebRtcServer(s, server)
	return s, server
}

type Server struct {
	pb.UnimplementedWebRtcServer
	peerAddr             string
	peerConnection       *webrtc.PeerConnection
	pendingIceCandidates *[]*webrtc.ICECandidate
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

// End boilerplate

// AddICECandidate adds the candidate to the peer connection's remote description
// This is called by both ends whenever a new candidate is discovered
// The remote description cannot be empty when this is called
func (s *Server) AddIceCandidate(ctx context.Context, candidate *pb.IceCandidate) (*pb.IceCandidateResponse, error) {
	log.Println("Receiving candidate signal")

	peerConnection := s.peerConnection
	log.Println("Received ICE candidate:", candidate)

	c := webrtc.ICECandidateInit{Candidate: candidate.JsonCandidate}
	// if onICECandidateErr := signalCandidate(*offerAddr, c); onICECandidateErr != nil {
	// panic(onICECandidateErr)
	log.Println("Adding ice candidate to peer connection")
	err := peerConnection.AddICECandidate(c)
	if err != nil {
		panic(err)
	}

	response := pb.IceCandidateResponse{}
	return &response, nil
}

// SdpSignal receves an offer from the client and returns a provisional answer.
// The peer connection object should already exist by the time this is called.
// The remote description is set to the offer, and data channel handlers are set up.
// The answer is returned, and *then* the local description is set, because once set,
// it kicks off the ICE candidate discovery process, which has an event handler to signal
// the other end (via gRPC) whenever a new candidate is found. The other end needs
// to have its remote description set before the AddIceCandidate function is called, so
// this order helps prevent race conditions
func (s *Server) SdpSignal(ctx context.Context, offer *pb.SdpOffer) (*pb.SdpAnswer, error) {
	// TODO handle pendingIceCandidates here
	log.Println("Received SDP Offer:", offer)

	// Set remote description based on incoming SDP
	sdp := webrtc.SessionDescription{}
	log.Println("Empty SDP", &sdp)
	if err := json.NewDecoder(strings.NewReader(offer.Offer)).Decode(&sdp); err != nil {
		panic(err)
	}
	log.Println("Decoded SDP", &sdp)
	peerConnection := s.peerConnection
	err := peerConnection.SetRemoteDescription(sdp)
	log.Printf("Remote description set")
	if err != nil {
		panic(err)
	}

	// Data channel will be created by other end
	// Add data channel handlers
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		log.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(onDataChannelOpen(d))

		// Register text message handling
		d.OnMessage(onDataChannelMessage(d))
	})

	// Create an answer (SDP) to send to the other end
	pranswer, err := peerConnection.CreateAnswer(nil)
	log.Println("created provisional answer:", pranswer)
	if err != nil {
		panic(err)
	}

	// Set local description *after* responding with pranswer
	defer func() {
		// Sets the LocalDescription, and starts our UDP listeners
		// Ensure this happens AFTER SetRemoteDescription because AddICECandidate
		// will break if remote description is not set
		// Also, ensure this happens AFTER SetRemoteDescription on the remote
		// because signaling an ICE candidate will try to AddICECandidate
		err = peerConnection.SetLocalDescription(pranswer)
		log.Println("local description set to provisional answer")
		if err != nil {
			panic(err)
		}

		log.Printf("SdpSignal: %v pendingIceCandidates", len(*s.pendingIceCandidates))
		// TODO wrap in mutex
		for _, c := range *s.pendingIceCandidates {
			if onICECandidateErr := signalCandidate(s.peerAddr, c); onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
	}()
	// Send our answer to the gRPC server on the other end
	payload, err := json.Marshal(pranswer)
	if err != nil {
		panic(err)
	}

	log.Println("responding with provisional answer:", string(payload))
	a := pb.SdpAnswer{Answer: string(payload)}
	return &a, nil
}

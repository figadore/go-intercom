package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pion/webrtc/v3"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/webrtc/pb"
)

const (
	port = ":20000"
)

func NewServer(host string) (*grpc.Server, *webrtc.PeerConnection) {
	s := grpc.NewServer()
	addr := fmt.Sprintf("192.168.0.%v%v", host, port)
	peerConnection, pendingCandidates := GetPeerConnection(addr)
	pb.RegisterWebRtcServer(s, &Server{
		pendingIceCandidates: pendingCandidates,
		peerConnection:       peerConnection,
	})
	return s, peerConnection
}

type Server struct {
	pb.UnimplementedWebRtcServer
	peerConnection       *webrtc.PeerConnection
	pendingIceCandidates []*webrtc.ICECandidate
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

// Add ICE candidate to remote description
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

func (s *Server) SdpSignal(ctx context.Context, offer *pb.SdpOffer) (*pb.SdpAnswer, error) {
	log.Println("Received SDP Offer:", offer)

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

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		log.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			log.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

			for range time.NewTicker(5 * time.Second).C {
				message := time.Now().String()
				log.Printf("Sending '%s'\n", message)

				// Send the message as text
				sendTextErr := d.SendText(message)
				if sendTextErr != nil {
					panic(sendTextErr)
				}
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			log.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})

	// Create an answer to send to the other process
	answer, err := peerConnection.CreateAnswer(nil)
	log.Println("created empty answer:", answer)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	log.Println("local description set:", answer)
	if err != nil {
		panic(err)
	}

	// Send our answer to the HTTP server listening in the other process
	payload, err := json.Marshal(answer)
	if err != nil {
		panic(err)
	}

	log.Println("responding with answer:", string(payload))
	a := pb.SdpAnswer{Answer: string(payload)}
	return &a, nil
}

package webrtc

import (
	"context"
	//"encoding/json"
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/webrtc/pb"
)

func signalCandidate(addr string, c *webrtc.ICECandidate) error {
	log.Println("Dialing grpc server", addr)
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewWebRtcClient(conn)
	ctx := context.Background()
	// End boilerplate

	payload := c.ToJSON().Candidate
	log.Println("Signaling candidate")
	result, err := client.AddIceCandidate(ctx, &pb.IceCandidate{JsonCandidate: payload})
	if err != nil {
		return err
	}
	log.Println("Result: ", result)
	return nil
}

func GetPeerConnection(addr string) (*webrtc.PeerConnection, []*webrtc.ICECandidate) {
	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)
	// When an ice candidate is discovered, let the other end know to update its remote description
	// which means remote description has to be set first
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		log.Println("ICE candidate found")
		if c == nil {
			log.Println("But found ICE candidate was nil")
			return
		}
		log.Printf("ICE candidate found: %s\n", c.String())

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		// If sdp hasn't been received yet, add this candidate to the pending list
		if peerConnection.RemoteDescription() == nil {
			log.Printf("OnICECandidate: desc is nil")
			pendingCandidates = append(pendingCandidates, c)
			return
		}
		// If sdp has been received, update the other side with this new candidate
		// log.Printf("OnICECandidate: RemoteDescription: %v\n", desc)
		log.Printf("OnICECandidate: signaling")
		err := signalCandidate(addr, c)
		if err != nil {
			panic(err)
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	return peerConnection, pendingCandidates
}

//func InitiateSdpSignaling(serverAddr string) string {
//	peerConnection, pendingCandidates := GetPeerConnection(serverAddr)
//	log.Println(pendingCandidates)
//
//	// Create an offer to send to the other process
//	log.Println("Creating sdp offer")
//	offer, err := peerConnection.CreateOffer(nil)
//	if err != nil {
//		panic(err)
//	}
//
//	// Sets the LocalDescription, and starts our UDP listeners
//	// Note: this will start the gathering of ICE candidates
//	log.Println("Setting local description")
//	if err = peerConnection.SetLocalDescription(offer); err != nil {
//		panic(err)
//	}
//	// When an ICE candidate is available send to the other Pion instance
//	// the other Pion instance will add this candidate by calling AddICECandidate
//
//	// Send our offer to the HTTP server listening in the other process
//	payload, err := json.Marshal(offer)
//	if err != nil {
//		panic(err)
//	}
//	sdpOffer := &pb.SdpOffer{Offer: string(payload)}
//	log.Println("Sending offer", sdpOffer)
//	answer := offerSdp(serverAddr, sdpOffer)
//	log.Println("Got answer", answer)
//	// Block forever
//	select {}
//	return answer
//}

func offerSdp(addr string, offer *pb.SdpOffer) string {
	ctx := context.Background()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewWebRtcClient(conn)
	answer, err := client.SdpSignal(ctx, offer)
	if err != nil {
		panic(err)
	}
	log.Println("Answer: ", answer)
	return answer.Answer
}

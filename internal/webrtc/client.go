package webrtc

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	//"encoding/json"
	"fmt"
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

//func offerSdp(addr string, offer *pb.SdpOffer) string {
//	ctx := context.Background()
//	var opts []grpc.DialOption
//	opts = append(opts, grpc.WithInsecure())
//	opts = append(opts, grpc.WithBlock())
//	conn, err := grpc.Dial(addr, opts...)
//	if err != nil {
//		log.Fatalf("fail to dial: %v", err)
//	}
//	defer conn.Close()
//	client := pb.NewWebRtcClient(conn)
//	answer, err := client.SdpSignal(ctx, offer)
//	if err != nil {
//		panic(err)
//	}
//	log.Println("Answer: ", answer)
//	return answer.Answer
//}

func Call(host string, peerConnection *webrtc.PeerConnection) *webrtc.DataChannel {
	// We're the client, we initiate by sending an sdp offer to the server
	// Hopefully the server will have its own ice candidates by now?
	// The server responds to the client's offer with an answer, which may initially contain no ice candidates
	// The client creates the data channel, and the server has an OnDataChannel method
	// Data channel listeners are set up on the peer client so they can start doing things as soon as a connection is established
	// Getting ICE candidates should happen asynchronously, and every time a new one is found, it is sent to the other side with AddIceCandidate endpoint
	// The remote description needs to be set before an ice candidate can be added to the peer connection object
	port := "20000"
	addr := fmt.Sprintf("192.168.0.%v:%v", host, port)
	// go webrtc.InitiateSdpSignaling(addr)
	// Create an offer to send to the other process
	log.Println("Creating sdp offer")
	// peerConnection, pendingCandidates := webrtc.GetPeerConnection(addr)
	// log.Println(pendingCandidates)

	// log.Println(sdp)
	dc := getDataChannel(peerConnection)

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	log.Println("Setting local description")
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Send our offer to the HTTP server listening in the other process
	payload, err := json.Marshal(offer)
	if err != nil {
		panic(err)
	}
	sdpOffer := &pb.SdpOffer{Offer: string(payload)}
	log.Println("Signalling offer via gRPC:", offer)
	answer := offerSdp(addr, sdpOffer)
	sdp := webrtc.SessionDescription{}
	err = json.NewDecoder(strings.NewReader(answer.Answer)).Decode(&sdp)
	log.Println("Decoded SDP", &sdp)
	if err != nil {
		panic(err)
	}
	err = peerConnection.SetRemoteDescription(sdp)
	log.Println("Set remote description")
	if err != nil {
		panic(err)
	}
	return dc
}

func offerSdp(addr string, offer *pb.SdpOffer) *pb.SdpAnswer {
	ctx := context.Background()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Printf("fail to dial: %v", err)
		panic(err)
	}
	defer conn.Close()
	client := pb.NewWebRtcClient(conn)
	answer, err := client.SdpSignal(ctx, offer)
	if err != nil {
		panic(err)
	}
	log.Println("Answer: ", answer)
	return answer
}

func getDataChannel(peerConnection *webrtc.PeerConnection) *webrtc.DataChannel {
	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil) // TODO set options to allow unordered?
	if err != nil {
		panic(err)
	}
	dataChannel.OnOpen(onDataChannelOpen(dataChannel))

	// Register text message handling
	dataChannel.OnMessage(onDataChannelMessage(dataChannel))
	return dataChannel
}

func onDataChannelMessage(dataChannel *webrtc.DataChannel) func(webrtc.DataChannelMessage) {
	return func(msg webrtc.DataChannelMessage) {
		// log.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
		log.Printf("Message from DataChannel: '%s'\n", string(msg.Data))
	}
}

func onDataChannelOpen(dataChannel *webrtc.DataChannel) func() {
	return func() {
		log.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())

		for range time.NewTicker(5 * time.Second).C {
			message := time.Now().String()
			log.Printf("Sending '%s'\n", message)

			// Send the message as text
			sendTextErr := dataChannel.SendText(message)
			if sendTextErr != nil {
				panic(sendTextErr)
			}
		}
	}
}

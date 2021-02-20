package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/figadore/go-intercom/internal/webrtc"
	pion "github.com/pion/webrtc/v3"
	"google.golang.org/grpc"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/webrtc/pb"
)

func run(args []string) int {
	// Enable global debug logs
	log.EnableDebug()
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	mainContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)
	addr := args[1]
	grpcServer, peerConnection := webrtc.NewServer(addr)
	// Start serving on AddIceCandidate and SdpSignal endpoints
	go webrtc.Serve(grpcServer, errCh)
	if len(args) > 2 {
		call(addr, peerConnection)
	}

	var msg string
	var exitCode int
	select {
	case sig := <-sigCh:
		msg = fmt.Sprintf("Received system signal: %v", sig)
		exitCode = 2
		// In a separate goroutine, listen for a second OS signal
		go func() {
			<-sigCh
			fmt.Println("Error: Received 2nd system signal, hard exit")
			os.Exit(2)
		}()
	case <-mainContext.Done():
		msg = fmt.Sprintf("Main context cancelled: %v", mainContext.Err())
		exitCode = 0
	case err := <-errCh:
		msg = fmt.Sprintf("Closing from error: %v", err)
		exitCode = 1
	}
	log.Println(msg)
	return exitCode
}

func main() {
	os.Exit(run(os.Args))
}

func call(host string, peerConnection *pion.PeerConnection) *pion.DataChannel {
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
	sdp := pion.SessionDescription{}
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

func getDataChannel(peerConnection *pion.PeerConnection) *pion.DataChannel {
	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil) // TODO set options to allow unordered?
	if err != nil {
		panic(err)
	}
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())

		for range time.NewTicker(5 * time.Second).C {
			message := time.Now().String()
			fmt.Printf("Sending '%s'\n", message)

			// Send the message as text
			sendTextErr := dataChannel.SendText(message)
			if sendTextErr != nil {
				panic(sendTextErr)
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg pion.DataChannelMessage) {
		fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
	})
	return dataChannel
}

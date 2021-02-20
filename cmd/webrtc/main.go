package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/figadore/go-intercom/internal/webrtc"

	"github.com/figadore/go-intercom/internal/log"
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
	grpcServer, server := webrtc.NewServer(addr)
	// Start serving on AddIceCandidate and SdpSignal endpoints
	go webrtc.Serve(grpcServer, errCh)
	// When running purely from cli, if 2 args were added to the call to the
	// binary, immediately initiate a call from that device
	if len(args) > 2 {
		webrtc.Call(addr, server)
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

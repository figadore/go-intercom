package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/figadore/go-intercom/internal/log"
	"github.com/figadore/go-intercom/internal/rpc"
	"github.com/figadore/go-intercom/internal/station"
)

func main() {
	// Enable global debug logs
	log.EnableDebug()
	// Handle externally generated OS exit signals
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start a parent context that can stop child processes on global error (errCh)
	ctx, cancel := context.WithCancel(context.Background())

	// Create a global fatal error channel
	errCh := make(chan error)

	// Get io specific to this station
	dotEnv, err := godotenv.Read()
	if err != nil {
		panic(err)
	}

	grpcServer := rpc.NewServer()
	// Create a grpc call manager to create new clients for outgoing calls
	callManager := rpc.NewCallManager()
	intercom := station.New(ctx, dotEnv, callManager)
	// Inject call manager so we can track and hang up calls initiated from other stations
	callManager.SetStation(intercom)
	defer log.Debugln("main: Closed intercom")
	defer intercom.Close()
	defer log.Debugln("main: Closing intercom")

	// Start the main process
	go rpc.Serve(grpcServer, errCh)

	// Do this last so that the context is cancelled *before* intercom.Close,
	// which has eventHandlers running that will block until the handler exist
	// The handler has a channel select on this context's Done() channel
	defer log.Debugln("Cancelled main context")
	defer cancel()
	defer log.Debugln("Cancelling main context")
	//if len(os.Args) > 1 {
	//	callManager.CallAll(ctx)
	//}
	// Run forever, but clean up on error or OS signals
	for {
		select {
		case sig := <-sigCh:
			msg := fmt.Sprintf("Received system signal: %v", sig)
			log.Println(msg)
			panic(msg)
		case err := <-errCh:
			log.Printf("Closing from error: %v", err)
			panic(err)
		}
	}
}

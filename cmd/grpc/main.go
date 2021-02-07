package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/figadore/go-intercom/internal/rpc"
	"github.com/figadore/go-intercom/internal/station"
)

func main() {
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
	callManager.SetStation(intercom)
	defer intercom.Close()

	// Start the main process
	// TODO inject call manager so we can track and hang up calls initiated from other stations
	go rpc.Serve(grpcServer, errCh)
	// go grpcServer.serve(grpcServer, errCh, intercom)

	defer cancel()
	// Wait for errors or OS signals
	for {
		select {
		case sig := <-sigCh:
			msg := fmt.Sprintf("Received system signal: %v", sig)
			log.Println(msg)
			panic(msg)
		case err := <-errCh:
			log.Printf("Closing from error: %v", err)
			panic(err)
			// case <-ctx.Done():
			//	log.Println("Exiting cleanly")
			//	return
		}
	}
}

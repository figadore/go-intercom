package main

import (
	"context"
	"log"

	"github.com/figadore/go-intercom/pb"
)

func startStreaming(ctx context.Context, stream pb.Receiver_AudioClient) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			err := sendAudio(stream)
			if err != nil {
				log.Printf("Error sending audio bytes: %v", err)
				return err
			}
		}
	}
}

func sendAudio(stream pb.Receiver_AudioClient) error {
	bytes := pb.AudioData{
		Data: []byte{4, 2, 0},
	}
	err := stream.Send(&bytes)
	if err != nil {
		return err
	}
	return nil
}

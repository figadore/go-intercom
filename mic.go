package main

import (
	"context"
	"log"
	//	"math"

	"github.com/figadore/go-intercom/pb"
	//	"github.com/jfreymuth/pulse"
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
		Data: []float32{4, 2, 0},
	}
	err := stream.Send(&bytes)
	if err != nil {
		return err
	}
	return nil
}

//var t, phase float32
//
//func synth(out []float32) (int, error) {
//	for i := range out {
//		if t > 4 {
//			return i, pulse.EndOfData
//		}
//		x := float32(math.Sin(2 * math.Pi * float64(phase)))
//		out[i] = x * 0.1
//		f := [...]float32{440, 550, 660, 880}[int(2*t)&3]
//		phase += f / 44100
//		if phase >= 1 {
//			phase--
//		}
//		t += 1. / 44100
//	}
//	return len(out), nil
//}

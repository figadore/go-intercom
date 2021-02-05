package main

import (
	"context"
	"log"
	"math"
	"sync"

	"github.com/figadore/go-intercom/pb"
	"github.com/jfreymuth/pulse"
)

func startStreaming(ctx context.Context, stream pb.Receiver_AudioClient, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Context.Done: %v", ctx.Err())
			errCh <- ctx.Err()

		default:
			i, err := sendAudio(stream)
			if err != nil {
				if err == pulse.EndOfData {
					log.Printf("End of streaming data: %v bytes", i)
				} else {
					errCh <- err
				}
			}
		}
	}
}

type sendBuffer struct {
	sync.Mutex
	bytes []float32
}

var t, phase float32

func (b *sendBuffer) fill() (int, error) {
	for i := range b.bytes {
		if t > 4 {
			return i, pulse.EndOfData
		}
		x := float32(math.Sin(2 * math.Pi * float64(phase)))
		b.bytes[i] = x * 0.05
		f := [...]float32{440, 550, 440, 880}[int(2*t)&3]
		phase += f / 44100
		if phase >= 1 {
			phase--
		}
		t += 1. / 44100
	}
	return len(b.bytes), nil
}

func sendAudio(stream pb.Receiver_AudioClient) (int, error) {
	micBuffer := sendBuffer{
		bytes: make([]float32, 256),
	}
	i, err := micBuffer.fill()
	if err != nil {
		return i, err
	}
	bytes := pb.AudioData{
		Data: micBuffer.bytes,
	}
	err = stream.Send(&bytes)
	if err != nil {
		return 0, err
	}
	return 0, nil
}

package station

import (
	"context"
	"time"

	"github.com/jfreymuth/pulse"

	"github.com/figadore/go-intercom/internal/log"
)

const (
	FragmentSize = 1600
	SampleRate   = 8000
)

type Speaker struct {
	AudioCh  chan []float32
	buffered []float32
	Context  context.Context
}

func (s *Speaker) Close() {
	close(s.AudioCh)
}

// startPlayback receives data from a channel and plays through the speaker
// func (speaker *Speaker) StartPlayback(ctx context.Context, errCh chan error) {
func (speaker *Speaker) StartPlayback(errCh chan error) {
	log.Println("startPlayback: enter")
	defer log.Println("startPlayback: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startPlayback: error creating pulse client", err)
		select {
		case errCh <- err:
			log.Println("startPlayback: sent pulse client creation error:", err)
		case <-time.After(5 * time.Second):
			log.Println("WARN: startSending: timeout pulse client creation error:", err)
		}
		return
	}
	defer c.Close()
	speakerStream, err := c.NewPlayback(pulse.Float32Reader(speaker.Read), pulse.PlaybackSampleRate(SampleRate), pulse.PlaybackBufferSize(FragmentSize))
	if err != nil {
		log.Println("startPlayback: error creating speaker stream", err)
		select {
		case errCh <- err:
			log.Println("startPlayback: sent speakerstream creation error:", err)
		case <-time.After(5 * time.Second):
			log.Println("WARN: startSending: timeout sending speakerstream creation error:", err)
		}
		return
	}
	defer log.Println("startPlayback: speakerStream closed")
	defer speakerStream.Close()
	defer log.Println("startPlayback: speakerStream stopped")
	defer speakerStream.Stop()
	speakerStream.Start()
	//// Stream to speaker until context is cancelled
	//<-ctx.Done()
	//log.Println("startPlayback: context done:", ctx.Err())
	// to allow Drain() to return, send pulse.EndOfData from reader
	// this should happen when the main context is cancelled for any reason
	// so the speaker stream will remain streaming until program close/crash
	log.Debugln("startPlayback: Draining speaker stream2")
	speakerStream.Drain()
	log.Debugln("startPlayback: Drained speaker stream2")
	log.Println("Underflow:", speakerStream.Underflow())
	if speakerStream.Error() != nil {
		err = speakerStream.Error()
		log.Println("startPlayback: speakerStream error", err)
		select {
		case errCh <- err:
			log.Println("startPlayback: speakerstream error:", err)
		case <-time.After(5 * time.Second):
			log.Println("WARN: startSending: timeout sending speakerstream error:", err)
		}
		return
	}
}

// Read receives data from the audio data channel and sends it to the speaker
//
// The length of buf is unknown, so we use the speaker's "buffered" field
// to hold whatever that doesn't fit in the current slice
//
// If what is already buffered isn't enough to fill buf, just copy that many
// bytes and return. The next Read will receive from the audio channel
func (s *Speaker) Read(buf []float32) (n int, err error) {
	select {
	case <-s.Context.Done():
		// close(s.audioCh)
		err = pulse.EndOfData
		log.Println("Speaker.Read: station.Context.Done(), sending EndOfData error", err)
		return
	default:
		break
	}
	if len(s.buffered) == 0 {
		// receives from the audio channel and places it in the buffer
		// blocking
		// TODO check for closed channel

		select {
		case data := <-s.AudioCh:
			s.buffered = append(s.buffered, data...)
		case <-time.After(5 * time.Second):
			log.Println("WARN: speaker.Read: timeout receiving from speaker.AudioCh")
			err = pulse.EndOfData
			return
		}
	}
	// Copies as much as possible, based on smaller of the two slices
	copy(buf, s.buffered)
	if len(s.buffered) >= len(buf) {
		n = len(buf)
	} else {
		n = len(s.buffered)
	}
	// Truncate data that has already been sent to the channel
	// Save the rest for the next call to Read
	s.buffered = s.buffered[n:]
	return
}

type Microphone struct {
	AudioCh chan []float32
	Context context.Context
}

func (m *Microphone) Close() {
	close(m.AudioCh)
}

// Write sends the data from the microphone buffer and sends it to the audio data channel
func (m *Microphone) Write(buf []float32) (n int, err error) {
	select {
	case <-m.Context.Done():
		// close(m.audioCh)
		return n, pulse.EndOfData
	default:
		break
	}
	data := make([]float32, len(buf))
	copy(data, buf)

	select {
	case m.AudioCh <- data:

	case <-time.After(5 * time.Second):
		log.Println("WARN: mic.Write: timeout sending to mic.AudioCh")
	}
	n = len(buf)
	return n, nil
}

// startRecording gets data from the microphone and sends it to the audio channel
func (mic *Microphone) StartRecording(ctx context.Context, errCh chan error) {
	log.Println("startRecording: enter")
	defer log.Println("startRecording: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startRecording: error creating pulse client", err)
		select {
		case errCh <- err:
			log.Println("startRecording: Sent error pulse client creation error", err)
		case <-time.After(5 * time.Second):
			log.Println("WARN: startRecording: timeout sending pulse client creation error", err)
		}
		return
	}
	defer c.Close()
	micStream, err := c.NewRecord(pulse.Float32Writer(mic.Write), pulse.RecordSampleRate(SampleRate), pulse.RecordBufferFragmentSize(FragmentSize))
	if err != nil {
		log.Println("startRecording: error creating new recorder", err)
		select {
		case errCh <- err:
			log.Println("startRecording: Sent error recorder creation error", err)
		case <-time.After(5 * time.Second):
			log.Println("WARN: startRecording: timeout sending recorder creation error", err)
		}
		return

	}
	defer log.Println("startRecording: micStream closed")
	defer micStream.Close()
	defer log.Println("startRecording: micStream stopped")
	defer micStream.Stop()
	micStream.Start() // async
	// Record until call ends
	<-ctx.Done()
	log.Println("startRecording: context done with error:", ctx.Err())
	//if err != nil {
	//	//log.Println("startRecording: sending error", err)
	//	//errCh <- err
	//	//log.Println("startRecording: sent error", err)
	//	return
	//}
}

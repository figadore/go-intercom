package station

import (
	"context"

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
	// TODO find out if this is necessary
	close(s.AudioCh)
}

// startPlayback receives data from a channel and plays through the speaker
func (speaker *Speaker) StartPlayback(ctx context.Context, errCh chan error) {
	log.Println("startPlayback: enter")
	defer log.Println("startPlayback: exit")
	c, err := pulse.NewClient()
	if err != nil {
		log.Println("startPlayback: error creating pulse client", err)
		errCh <- err
		return
	}
	defer c.Close()
	speakerStream, err := c.NewPlayback(pulse.Float32Reader(speaker.Read), pulse.PlaybackSampleRate(SampleRate), pulse.PlaybackBufferSize(FragmentSize))
	if err != nil {
		log.Println("startPlayback: error creating speaker stream", err)
		errCh <- err
		return
	}
	defer speakerStream.Close()
	speakerStream.Start()
	// Stream to speaker until context is cancelled
	<-ctx.Done()
	log.Println("startPlayback: context done")
	errCh <- ctx.Err()
	if err != nil {
		log.Println("startPlayback: context error", err)
		errCh <- err
		// to allow Drain() to return, send pulse.EndOfData from reader
		// this should happen when the context is cancelled for any reason (see Speaker.Read)
		log.Debugln("Debug: Draining speaker stream")
		speakerStream.Drain()
		log.Debugln("Debug: Drained speaker stream")
		return
	}
	log.Println("Underflow:", speakerStream.Underflow())
	if speakerStream.Error() != nil {
		err = speakerStream.Error()
		log.Println("startPlayback: speakerStream error", err)
		errCh <- err
		log.Println("startPlayback: sent speakerStream error", err)
		return
	}
}

// Read receives data from the audio data channel and sends it to the speaker
// TODO document why an intermediate buffer is needed (s.buffered) and why it doesn't apply to mic.Write
func (s *Speaker) Read(buf []float32) (n int, err error) {
	select {
	case <-s.Context.Done():
		// close(s.audioCh)
		return n, pulse.EndOfData
	default:
		break
	}
	if len(s.buffered) == 0 {
		// receives from the audio channel and places it in the buffer
		// blocking
		// TODO check for closed channel
		data := <-s.AudioCh
		s.buffered = append(s.buffered, data...)
	}
	copy(buf, s.buffered)
	if len(s.buffered) >= len(buf) {
		n = len(buf)
	} else {
		n = len(s.buffered)
	}
	s.buffered = s.buffered[n:]
	return
}

type Microphone struct {
	AudioCh chan []float32
	Context context.Context
}

func (m *Microphone) Close() {
	// TODO find out if this is necessary
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
	m.AudioCh <- data
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
		errCh <- err
		return
	}
	defer c.Close()
	micStream, err := c.NewRecord(pulse.Float32Writer(mic.Write), pulse.RecordSampleRate(SampleRate), pulse.RecordBufferFragmentSize(FragmentSize))
	if err != nil {
		log.Println("startRecording: error creating new recorder", err)
		errCh <- err
		return

	}
	defer micStream.Close()
	micStream.Start() // async
	<-ctx.Done()
	log.Println("startRecording: context done")
	err = ctx.Err()
	if err != nil {
		log.Println("startRecording: context error", err)
		micStream.Stop()
		log.Println("startRecording: sending error", err)
		errCh <- err
		log.Println("startRecording: sent error", err)
		return
	}
}

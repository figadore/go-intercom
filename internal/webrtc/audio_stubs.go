package webrtc

import (
	"log"
	"reflect"
	"time"
	"unsafe"

	"github.com/jfreymuth/pulse"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

const (
	channels      = 1
	SampleRate    = 8000
	frameSize     = 800
	FragmentSize  = 1600
	frameDuration = (time.Second / SampleRate) * frameSize
)

//// TODO make these real values
const payloadType = 111

//var audioCodec = webrtc.RTPCodecCapability{
//	MimeType:  "audio/speex",
//	ClockRate: 8000,
//	Channels:  1,
//}

type Speaker struct {
	AudioCh  chan []float32
	buffered []float32
	done     chan struct{}
}

func float32Slice(s []byte) []float32 {
	h := *(*reflect.SliceHeader)(unsafe.Pointer(&s))
	return *(*[]float32)(unsafe.Pointer(&reflect.SliceHeader{Data: h.Data, Len: h.Len / 4, Cap: h.Len / 4}))
}

func byteSlice(s []float32) []byte {
	h := *(*reflect.SliceHeader)(unsafe.Pointer(&s))
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: h.Data, Len: h.Len * 4, Cap: h.Len * 4}))
}

// Read receives data from the audio data channel and sends it to the speaker
//
// The length of buf is unknown, so we use the speaker's "buffered" field
// to hold whatever that doesn't fit in the current slice
//
// If what is already buffered isn't enough to fill buf, just copy that many
// bytes and return. The next Read will receive from the audio channel
func (s *Speaker) Read(buf []float32) (n int, err error) {
	log.Println("Speaker writing")
	select {
	case <-s.done:
		// close(s.audioCh)
		err = pulse.EndOfData
		log.Println("Speaker.Read: done channel closed, sending EndOfData error", err)
		return
	default:
		break
	}
	if len(s.buffered) == 0 {
		// receives from the audio channel and places it in the buffer
		// TODO check for closed channel

		select {
		case <-s.done:
			// close(s.audioCh)
			err = pulse.EndOfData
			log.Println("Speaker.Read: done channel closed2, sending EndOfData error", err)
			return
		case data := <-s.AudioCh:
			log.Println("Speaker  received the following data from AudioCh:", data)
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
	done    chan struct{}
}

// Write sends the data from the microphone buffer and sends it to the audio data channel
func (m *Microphone) Write(buf []float32) (n int, err error) {
	log.Println("Mic reading")
	select {
	case <-m.done:
		// close(m.audioCh)
		return n, pulse.EndOfData
	default:
		break
	}
	data := make([]float32, len(buf))
	copy(data, buf)

	select {
	case m.AudioCh <- data:
		log.Println("Mic sending the following data to AudioCh:", data)
		return n, pulse.EndOfData
	case <-m.done:
		// close(m.audioCh)
		return n, pulse.EndOfData

	case <-time.After(5 * time.Second):
		log.Println("WARN: mic.Write: timeout sending to mic.AudioCh")
	}
	n = len(buf)
	return n, nil
}

// startRecording gets data from the microphone and sends it to the audio channel
func (mic *Microphone) beginRecording(audioTrack *webrtc.TrackLocalStaticSample) {
	log.Println("startRecording: enter")
	defer log.Println("startRecording: exit")
	c, err := pulse.NewClient()
	if err != nil {
		panic(err)
	}
	defer c.Close()
	micStream, err := c.NewRecord(pulse.Float32Writer(mic.Write), pulse.RecordSampleRate(SampleRate), pulse.RecordBufferFragmentSize(FragmentSize))
	if err != nil {
		panic(err)
	}
	log.Println("startRecording: created mic stream")
	defer log.Println("startRecording: micStream closed")
	defer micStream.Close()
	defer log.Println("startRecording: micStream stopped")
	defer micStream.Stop()
	log.Println("startRecording: starting mic stream")
	micStream.Start() // async
	log.Println("startRecording: started mic stream, waiting for ctx.Done()")
	for i := 0; i < 11; i++ {
		audioData := <-mic.AudioCh
		log.Println("Received the following data from mic's AudioCh:", audioData)
		audioBytes := byteSlice(audioData)
		log.Println("Mic's data as a byteslice:", audioBytes)
		err = audioTrack.WriteSample(media.Sample{
			Data:     audioBytes,
			Duration: frameDuration,
		})
		if err != nil {
			panic(err)
		}
	}
	// Record until call ends
	//if err != nil {
	//	//log.Println("startRecording: sending error", err)
	//	//errCh <- err
	//	//log.Println("startRecording: sent error", err)
	//	return
	//}
}

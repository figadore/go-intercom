# Go Intercom (for Raspberry Pi)

## Build
`./build.sh`

## Deploy
run `./deploy.sh <host>` to copy the binaries to the device

## Run
### Setup
copy <repo>/.env.dist to <remote>:<dir>.env (currently only works with run.sh if at root of home)

set variables in .env in the directory where binaries are deployed (see github.com/joho/godotenv)

### Run

run `./run.sh <host>` to run the binary through ssh

## Structure
An intercom [Station] represents a Raspberry Pi and some logical components. A station has:
* An [Inputs] object, representing the physical (or virtual) input user interface, such as switches and buttons, a touchscreen menu, voice commands, or even command line
* An [Outputs] object, representing the physical feedback user interface, such as LEDs, a display, text-to-speech, etc
* A [CallManager] which tracks and handles interactions with all incoming and outgoing calls
* A [Speaker] which outputs audio, but also handles sound mixing, filters, and other pipelines
* A [Microphone] which receives input audio, but also handles audio pipelines

The Inputs should not talk directly to the CallManager, they should talk to the Station so that it can update the Display and do other high-level management operations

### gRPC
The cmd/grpc package creates the top-level [Station] object with a context that is cancelled on error (errCh) or OS signal. A gRPC server is created, and input handlers are set up to create new gRPC clients when activated. The gRPC server listens in a go routine, and waits for context cancel or error.

### Station
The intercom station object is the primary point of contact for various components.

#### LED Outputs
* Green LED: call connected
* Yellow LED: auto-answer off (aka do-not-disturb)
* Flashing Yellow LED: outgoing call pending, other side has no auto-answer or is not online
* Flashing Green LED: incoming call. Accept or reject, or 20 second to auto-reject
* Green and Yellow at the same time: Error

### Calls
[Call]s are managed by the [CallManager]. Whether a call is incoming or outoing, the same duplexCall function is used (though this might change when multi-way calling is added). Each call object has it's own context and cancel method, so that it can be cancelled from the inputs through the call manager

# TODO

## reproduce
* call manager removes call from list, or call.hangup through callmananger?
  * add pointer in call struct to call manager? or at least a callback when when cancel is called?

## eventually
* fix compounding lag
* try out webrtc for conference calling
* allow multiple audio streams, separate speaker object from buffers (attach buffers to Call?)
  * make startSending/startReceiving part of the call object?

## CD
* add a way to interact with running programs (cli version of buttons)
* unit and functional tests

* daemonize
* run on startup

# Changelog
* handle second sigint with immediate hard exit
* fix pending call blinking, stop when call accepted or rejected
* fix mic starts recording while call in pending on dnd side
* fix reject call while in dnd
* fix blinking light on dnd server when client ends before server accepts or rejects

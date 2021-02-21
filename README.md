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

### WebRTC
The cmd/webrtc package does stuff
The initial connectivity is accomplished using gRPC to signal peers about the SDP and ICE candidates. Using pion, the order is important.

On both the client and the server, when an ICE candidate is found, the OnICECandidate callback runs, which calls the AddIceCandidate gRPC call. In order for the other side to add the ICE candidate, the client's and server's remote description need to be set on the peerConnection. The client side delays the need to have the remote description set by collecting a slice of `pendingIceCandidates`, which is iterated after it receives the pranswer

1. The "client" or initiator creates a minimal "offer" SDP
1. The client sets the local description to the offer (This kicks off the ICE detection process on the client. ICE candidates discovered during this period are added to the `pendingIceCandidates` slice)
1. The client sends the offer to the "server" using gRPC
1. The server sets the remote description to the offer
1. The server creates a provisional answer (pranswer)
1. The client receives the (pranswer) as a response to the gRPC call
1. The client sets the remote description to the pranswer
1. The server sets the local description to the pranswer (This kicks off the ICE detection process on the server)
1. The client iterates the `pendingIceCandidates` and sends them to the server via the SdpSignal gRPC call. The server alls AddIceCandidate on the peerConnection. Any future discovered ICE candidates follow this same process
1. Any ICE candidates discovered on the server side send them to the client via the same SdpSignal gRPC call.
1. A connection is established
1. The client creates the data channel

```
     +----+                                                        +------+                            +------+                        
     ¦stun¦                                                        ¦client¦                            ¦server¦                        
     +----+                                                        +------+                            +------+                        
       ¦ setLocalDescription(offer), kicks off ice candidate discovery¦                                   ¦                            
       ¦ <-------------------------------------------------------------                                   ¦                            
       ¦                                                              ¦                                   ¦                            
       ¦                       new ice candidate                      ¦                                   ¦                            
       ¦ ------------------------------------------------------------->                                   ¦                            
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦----+                              ¦                            
       ¦                                                              ¦    ¦ add pending ice candidate    ¦                            
       ¦                                                              ¦<---+                              ¦                            
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦        signal offer (/sdp)        ¦                            
       ¦                                                              ¦ ---------------------------------->                            
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦                                   ¦----+                       
       ¦                                                              ¦                                   ¦    ¦ set remote description
       ¦                                                              ¦                                   ¦<---+                       
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦           signal answer           ¦                            
       ¦                                                              ¦ <----------------------------------                            
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦----+                                                           
       ¦                                                              ¦    ¦ setRemoteDescription(pranswer)                            
       ¦                                                              ¦<---+                                                           
       ¦                                                              ¦                                   ¦                            
       ¦                      setLocalDescription(pranswer), kicks off ice discovery                      ¦                            
       ¦ <-------------------------------------------------------------------------------------------------                            
       ¦                                                              ¦                                   ¦                            
       ¦                                         new ice candidate    ¦                                   ¦                            
       ¦ ------------------------------------------------------------------------------------------------->                            
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦      signal ice (/candidate)      ¦                            
       ¦                                                              ¦ <----------------------------------                            
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦----+                                                           
       ¦                                                              ¦    ¦ iterate pending ice candidates                            
       ¦                                                              ¦<---+                                                           
       ¦                                                              ¦                                   ¦                            
       ¦                                                              ¦      signal ice (/candidate)      ¦                            
       ¦                                                              ¦ ---------------------------------->                            
     +----+                                                        +------+                            +------+                        
     ¦stun¦                                                        ¦client¦                            ¦server¦                        
     +----+                                                        +------+                            +------+     
```


```
@startuml
stun <- client: setLocalDescription(offer), kicks off ice candidate discovery
stun -> client: new ice candidate
client -> client: add pending ice candidate
client -> server: signal offer (/sdp)
server -> server: set remote description
server -> client: signal answer
client -> client: setRemoteDescription(pranswer)
server -> stun: setLocalDescription(pranswer), kicks off ice discovery
stun-> server: new ice candidate
server -> client: signal ice (/candidate)
client -> client: iterate pending ice candidates
client -> server: signal ice (/candidate)
@enduml
```

Note: there is a small period of time between when the client has received the pranswer and when the client has set the local description in which ICE candidates from the server may arrive. TODO: solve this

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

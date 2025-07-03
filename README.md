# DirectChan
### Description
`DirectChan` is a minimal Go library implementing the webrtc protocol for peer communication. It is based on the [Pion/webrtc](https://github.com/pion/webrtc) library and aims to serve as a base for applications leveraging the WebRTC protocol.
The signaling is handled by a signaling server implemented in `server/main.go`.

### Usage
Server setup:
```
go build -o bin/server server/main.go
./bin/server/main <port>
```

Example setup:
```
go build -o bin/example example/main.go
```
Then make an offer for a connection called "secret-key":
```
./bin/example ws://<server-ip>:<port> offer "secret-key"
```
Lastly make an answer for the connection above:
```
./bin/example ws://<server-ip>:<port> answer "secret-key"
```
Note: prepare the two commands to run because the server keeps the key active for ten seconds.

### Future improvements
- WebSocket encryption with `wss` protocol support
- WebRTC stream encryption
- Media optimizations

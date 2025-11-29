# RPC Chat Realtime (Assignment 05)

Simple RPC-based chat with real-time broadcasting (Go).

Features:
- Central RPC server on :9000
- Multiple clients; each client exposes a small RPC endpoint to receive broadcasts
- When a client joins, server broadcasts "User [ID] joined" to others
- Messages are broadcast to all other clients (no self-echo)
- History returned on join
- Concurrency via goroutines and channels; clients map protected by Mutex

Run:
1. Start server:
   cd server
   go run server.go

2. Start clients (each in its own terminal):
   cd client
   go run client.go <clientID> <listenAddr>
   e.g. go run client.go alice :10001

Notes:
- To exit a client, type /quit or /exit
- Ensure ports you pick (client listen addrs) are free

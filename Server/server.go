package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

// Message represents a chat message.
type Message struct {
	SenderID  string
	Timestamp time.Time
	Text      string
}

type JoinArgs struct {
	ClientID string
	Addr     string // client's rpc listen address (host:port)
}

type JoinReply struct {
	History []Message
}

type SendArgs struct {
	SenderID string
	Text     string
}

type SendReply struct {
	Ok bool
}

type ChatServer struct {
	mu        sync.Mutex
	clients   map[string]*rpc.Client
	history   []Message
	broadcast chan Message
}

func NewChatServer() *ChatServer {
	s := &ChatServer{
		clients:   make(map[string]*rpc.Client),
		history:   make([]Message, 0),
		broadcast: make(chan Message, 256),
	}
	go s.broadcaster()
	return s
}

// Join: client provides its ID and address where it serves RPC
func (s *ChatServer) Join(args JoinArgs, reply *JoinReply) error {
	if args.ClientID == "" || args.Addr == "" {
		return fmt.Errorf("clientID and Addr required")
	}

	cli, err := rpc.Dial("tcp", args.Addr)
	if err != nil {
		return fmt.Errorf("cannot dial client at %s: %w", args.Addr, err)
	}

	s.mu.Lock()
	// if existing client with same ID, close previous connection
	if old, ok := s.clients[args.ClientID]; ok && old != nil {
		_ = old.Close()
	}
	s.clients[args.ClientID] = cli

	joinMsg := Message{
		SenderID:  "SERVER",
		Timestamp: time.Now().UTC(),
		Text:      fmt.Sprintf("User %s joined", args.ClientID),
	}
	s.history = append(s.history, joinMsg)

	// copy history to reply
	hcopy := make([]Message, len(s.history))
	copy(hcopy, s.history)
	s.mu.Unlock()

	// broadcast the join message (so other clients get "User X joined")
	s.broadcast <- joinMsg

	log.Printf("Client %s joined (%s). Total clients: %d", args.ClientID, args.Addr, len(s.clients))
	reply.History = hcopy
	return nil
}

func (s *ChatServer) GetHistory(_struct struct{}, reply *JoinReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hcopy := make([]Message, len(s.history))
	copy(hcopy, s.history)
	reply.History = hcopy
	return nil
}

func (s *ChatServer) SendMessage(args SendArgs, reply *SendReply) error {
	msg := Message{
		SenderID:  args.SenderID,
		Timestamp: time.Now().UTC(),
		Text:      args.Text,
	}
	// append to history and enqueue for broadcast
	s.mu.Lock()
	s.history = append(s.history, msg)
	s.mu.Unlock()

	s.broadcast <- msg
	reply.Ok = true
	return nil
}

// broadcaster reads from the broadcast channel and sends to all clients concurrently (no self-echo)
func (s *ChatServer) broadcaster() {
	for msg := range s.broadcast {
		s.mu.Lock()
		// snapshot clients to avoid holding lock for long
		clientsCopy := make(map[string]*rpc.Client, len(s.clients))
		for id, c := range s.clients {
			clientsCopy[id] = c
		}
		s.mu.Unlock()

		var wg sync.WaitGroup
		for id, c := range clientsCopy {
			// skip sending back to sender
			if id == msg.SenderID {
				continue
			}
			wg.Add(1)
			go func(clientID string, client *rpc.Client) {
				defer wg.Done()
				// Call client's Receive method
				var ack struct{}
				callErr := client.Call("Client.Receive", msg, &ack)
				if callErr != nil {
					log.Printf("error calling client %s: %v — removing client", clientID, callErr)
					// remove faulty client
					s.mu.Lock()
					if old, ok := s.clients[clientID]; ok {
						_ = old.Close()
						delete(s.clients, clientID)
					}
					s.mu.Unlock()
				}
			}(id, c)
		}
		// wait for all sends to attempt (so we don't flood)
		wg.Wait()
	}
}

func main() {
	server := NewChatServer()
	rpc.Register(server)

	listenAddr := ":9000" // central server port
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	log.Printf("Chat server listening on %s", listenAddr)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept err: %v", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}

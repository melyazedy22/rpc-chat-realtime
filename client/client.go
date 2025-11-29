package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"
)

// Message must match server.Message
type Message struct {
	SenderID  string
	Timestamp time.Time
	Text      string
}

type Client struct {
	ID string
}

func (c *Client) Receive(msg Message, _ *struct{}) error {
	// print incoming message (not from this client)
	// format: [time] sender: text
	fmt.Printf("\n[%s] %s: %s\n> ", msg.Timestamp.Format("15:04:05"), msg.SenderID, msg.Text)
	return nil
}

type JoinArgs struct {
	ClientID string
	Addr     string
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

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s <clientID> <clientListenAddr (e.g. :10001)>\n", os.Args[0])
		return
	}
	clientID := os.Args[1]
	listenAddr := os.Args[2]
	// start RPC server for this client
	clientObj := &Client{ID: clientID}
	rpc.Register(clientObj)

	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("client listen error: %v", err)
	}
	defer l.Close()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("client accept err: %v", err)
				return
			}
			go rpc.ServeConn(conn)
		}
	}()

	// connect to central server
	serverAddr := ":9000"
	srv, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("cannot connect to server at %s: %v", serverAddr, err)
	}
	// call Join
	var joinReply JoinReply
	joinArgs := JoinArgs{ClientID: clientID, Addr: listenAddr}
	if err := srv.Call("ChatServer.Join", joinArgs, &joinReply); err != nil {
		log.Fatalf("join failed: %v", err)
	}

	// print history
	fmt.Println("=== History ===")
	for _, m := range joinReply.History {
		fmt.Printf("[%s] %s: %s\n", m.Timestamp.Format("2006-01-02 15:04:05"), m.SenderID, m.Text)
	}
	fmt.Println("=== End History ===")

	// interactive send loop
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("> ")
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			fmt.Print("> ")
			continue
		}
		// local exit command
		if line == "/quit" || line == "/exit" {
			fmt.Println("exiting...")
			return
		}
		// send to server
		var sendReply SendReply
		sendArgs := SendArgs{SenderID: clientID, Text: line}
		if err := srv.Call("ChatServer.SendMessage", sendArgs, &sendReply); err != nil {
			log.Printf("send error: %v", err)
		}
		fmt.Print("> ")
	}
}

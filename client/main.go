package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mapreduce/pb"
)

type Client struct {
	id       string
	bossAddr string
	client   pb.ClientServiceClient
	stream   pb.ClientService_ClientControlClient
	ctx      context.Context
	cancel   context.CancelFunc
}

func main() {
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	bossAddr := "localhost:8080"

	log.Printf("Starting dummy client %s", clientID)

	client := &Client{
		id:       clientID,
		bossAddr: bossAddr,
	}

	if err := client.connect(); err != nil {
		log.Fatalf("Failed to connect to boss: %v", err)
	}

	client.run()
}

func (c *Client) connect() error {
	conn, err := grpc.Dial(c.bossAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial boss: %v", err)
	}

	c.client = pb.NewClientServiceClient(conn)
	c.ctx, c.cancel = context.WithCancel(context.Background())

	stream, err := c.client.ClientControl(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to create control stream: %v", err)
	}
	c.stream = stream

	// Send hello message
	hello := &pb.ClientToBoss{
		Msg: &pb.ClientToBoss_Hello{
			Hello: &pb.ClientHello{
				ClientId: c.id,
				UserName: "test-user",
			},
		},
	}

	if err := c.stream.Send(hello); err != nil {
		return fmt.Errorf("failed to send hello: %v", err)
	}

	log.Printf("Connected to boss and sent hello")
	return nil
}

func (c *Client) run() {
	// Start message handling
	go c.handleMessages()

	// Start interactive input loop
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("=== MapReduce Client ===")
	fmt.Println("Commands:")
	fmt.Println("  'a' + Enter: Submit a job")
	fmt.Println("  'q' + Enter: Quit")
	fmt.Print("> ")

	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		switch input {
		case "a":
			c.submitJob()
		case "q":
			log.Println("Quitting...")
			c.cancel()
			return
		default:
			fmt.Printf("Unknown command: %s\n", input)
		}

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

func (c *Client) handleMessages() {
	for {
		msg, err := c.stream.Recv()
		if err == io.EOF {
			log.Println("Boss disconnected")
			c.cancel()
			return
		}
		if err != nil {
			log.Printf("Error receiving from boss: %v", err)
			c.cancel()
			return
		}

		switch m := msg.Msg.(type) {
		case *pb.BossToClient_StatusDump:
			fmt.Printf("\n%s\n> ", m.StatusDump)
		}
	}
}

func (c *Client) submitJob() {

	mapperCount := 2
	reducerCount := 2

	jobRequest := &pb.ClientToBoss{
		Msg: &pb.ClientToBoss_SubmitJob{
			SubmitJob: &pb.SubmitJobRequest{
				CodeUri:        "/wordcount.py",
				InputFiles:     []string{"/input.csv"},
				MapperCount:    int32(mapperCount),
				ReducerCount:   int32(reducerCount),
				EnableCombiner: true,
				InputType:      pb.DataFormat_TEXT,
				OutputType:     pb.DataFormat_TEXT,
				OutputDir:      "/output",
			},
		},
	}

	if err := c.stream.Send(jobRequest); err != nil {
		log.Printf("Failed to submit job: %v", err)
		return
	}
}

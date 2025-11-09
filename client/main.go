package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

	// WebSocket server for dashboard connections
	wsUpgrader     websocket.Upgrader
	wsClients      map[*websocket.Conn]bool
	wsClientsMux   sync.RWMutex
	wsBroadcastMux sync.Mutex // Protect WebSocket writes
}

type FileSystemNode struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"` // "file" or "directory"
	Path     string            `json:"path"`
	Children []*FileSystemNode `json:"children,omitempty"`
}

type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type JobSubmissionRequest struct {
	CodeUri        string   `json:"codeUri"`
	InputFiles     []string `json:"inputFiles"`
	MapperCount    int32    `json:"mapperCount"`
	ReducerCount   int32    `json:"reducerCount"`
	EnableCombiner bool     `json:"enableCombiner"`
	InputType      string   `json:"inputType"`
	OutputType     string   `json:"outputType"`
	OutputDir      string   `json:"outputDir"`
}

type JobSubmissionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	JobId   string `json:"jobId,omitempty"`
}

type FileUploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

func main() {
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	bossAddr := "localhost:8080"

	client := &Client{
		id:       clientID,
		bossAddr: bossAddr,
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		wsClients: make(map[*websocket.Conn]bool),
	}

	if err := client.connect(); err != nil {
		log.Fatalf("Failed to connect to boss: %v", err)
	}

	// Start WebSocket server
	go client.startWebSocketServer()

	// Send initial volume directory structure
	go client.sendVolumeDirectoryPeriodically()

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

	// Keep the client running
	<-c.ctx.Done()
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
		case *pb.BossToClient_DashboardState:
			c.handleDashboardState(m.DashboardState)
		}
	}
}

func (c *Client) handleDashboardState(state *pb.DashboardState) {
	// Forward dashboard state to WebSocket clients
	dashboardMsg := WebSocketMessage{
		Type: "dashboard_state",
		Data: state,
	}
	c.broadcastWebSocketMessage(dashboardMsg)
}

// startWebSocketServer starts the WebSocket server for dashboard connections
func (c *Client) startWebSocketServer() {
	http.HandleFunc("/ws", c.handleWebSocketConnection)

	// Add job submission API endpoint
	http.HandleFunc("/api/submit-job", c.handleJobSubmission)

	// Add file upload API endpoint
	http.HandleFunc("/api/upload-file", c.handleFileUpload)

	// Add CORS headers for development
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Write([]byte("MapReduce Dashboard WebSocket Server"))
	})

	log.Println("Starting WebSocket server on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Printf("WebSocket server failed: %v", err)
	}
}

// handleWebSocketConnection handles new WebSocket connections from dashboards
func (c *Client) handleWebSocketConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := c.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	log.Printf("New dashboard connected: %s", r.RemoteAddr)

	// Add client to the list
	c.wsClientsMux.Lock()
	c.wsClients[conn] = true
	c.wsClientsMux.Unlock()

	// Handle disconnection
	defer func() {
		c.wsClientsMux.Lock()
		delete(c.wsClients, conn)
		c.wsClientsMux.Unlock()
		conn.Close()
		log.Printf("Dashboard disconnected: %s", r.RemoteAddr)
	}()

	// Keep connection alive and handle ping/pong
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// broadcastWebSocketMessage sends a message to all connected WebSocket clients
func (c *Client) broadcastWebSocketMessage(message WebSocketMessage) {
	// Serialize the broadcast to prevent concurrent WebSocket writes
	c.wsBroadcastMux.Lock()
	defer c.wsBroadcastMux.Unlock()

	if len(c.wsClients) == 0 {
		return // No clients connected
	}

	// Convert message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal WebSocket message: %v", err)
		return
	}

	c.wsClientsMux.RLock()
	defer c.wsClientsMux.RUnlock()

	// Send to all connected clients
	for client := range c.wsClients {
		err := client.WriteMessage(websocket.TextMessage, messageJSON)
		if err != nil {
			log.Printf("Failed to send to WebSocket client: %v", err)
			// Client will be removed on next read error
		}
	}
}

// sendVolumeDirectoryPeriodically scans and sends the volume directory structure periodically
func (c *Client) sendVolumeDirectoryPeriodically() {
	// Send initial directory structure
	c.sendVolumeDirectory()

	// Send updates every 3 seconds
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.sendVolumeDirectory()
		case <-c.ctx.Done():
			return
		}
	}
}

// sendVolumeDirectory scans the volume directory and sends the structure to WebSocket clients
func (c *Client) sendVolumeDirectory() {
	volumePath := os.Getenv("VOLUME_PATH")
	if volumePath == "" {
		volumePath = "/volume" // Default fallback
	}

	// Check if volume directory exists
	if _, err := os.Stat(volumePath); os.IsNotExist(err) {
		log.Printf("Volume directory does not exist: %s", volumePath)
		return
	}

	// Scan directory structure
	root, err := c.scanDirectory(volumePath, "")
	if err != nil {
		log.Printf("Failed to scan volume directory: %v", err)
		return
	}

	// Send the children directly instead of the root volume node
	var directoryContents []*FileSystemNode
	if root != nil && root.Children != nil {
		directoryContents = root.Children
	}

	// Send to WebSocket clients
	volumeMsg := WebSocketMessage{
		Type: "volume_directory",
		Data: directoryContents,
	}
	c.broadcastWebSocketMessage(volumeMsg)
}

// scanDirectory recursively scans a directory and builds a FileSystemNode tree
func (c *Client) scanDirectory(basePath, relativePath string) (*FileSystemNode, error) {
	fullPath := filepath.Join(basePath, relativePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	node := &FileSystemNode{
		Name: info.Name(),
		Path: relativePath,
	}

	if relativePath == "" {
		node.Name = "volume" // Root name
	}

	if info.IsDir() {
		node.Type = "directory"

		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			childPath := filepath.Join(relativePath, entry.Name())
			child, err := c.scanDirectory(basePath, childPath)
			if err != nil {
				log.Printf("Error scanning child %s: %v", childPath, err)
				continue // Skip problematic entries
			}
			node.Children = append(node.Children, child)
		}
	} else {
		node.Type = "file"
	}

	return node, nil
}

// handleJobSubmission handles REST API job submission requests
func (c *Client) handleJobSubmission(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(JobSubmissionResponse{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	// Parse the JSON request
	var jobReq JobSubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&jobReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JobSubmissionResponse{
			Success: false,
			Message: "Invalid JSON request: " + err.Error(),
		})
		return
	}

	// Validate required fields
	if jobReq.CodeUri == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JobSubmissionResponse{
			Success: false,
			Message: "CodeUri is required",
		})
		return
	}

	if len(jobReq.InputFiles) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JobSubmissionResponse{
			Success: false,
			Message: "InputFiles is required",
		})
		return
	}

	// Convert input type string to protobuf enum
	var inputType pb.DataFormat
	switch strings.ToUpper(jobReq.InputType) {
	case "TEXT":
		inputType = pb.DataFormat_TEXT
	case "JSON":
		inputType = pb.DataFormat_JSON
	case "PARQUET":
		inputType = pb.DataFormat_PARQUET
	default:
		inputType = pb.DataFormat_TEXT
	}

	// Convert output type string to protobuf enum
	var outputType pb.DataFormat
	switch strings.ToUpper(jobReq.OutputType) {
	case "TEXT":
		outputType = pb.DataFormat_TEXT
	case "JSON":
		outputType = pb.DataFormat_JSON
	case "PARQUET":
		outputType = pb.DataFormat_PARQUET
	default:
		outputType = pb.DataFormat_TEXT
	}

	// Create the job request
	grpcJobRequest := &pb.ClientToBoss{
		Msg: &pb.ClientToBoss_SubmitJob{
			SubmitJob: &pb.SubmitJobRequest{
				CodeUri:        jobReq.CodeUri,
				InputFiles:     jobReq.InputFiles,
				MapperCount:    jobReq.MapperCount,
				ReducerCount:   jobReq.ReducerCount,
				EnableCombiner: jobReq.EnableCombiner,
				InputType:      inputType,
				OutputType:     outputType,
				OutputDir:      jobReq.OutputDir,
			},
		},
	}

	// Send the job to the boss
	if err := c.stream.Send(grpcJobRequest); err != nil {
		log.Printf("Failed to submit job via API: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(JobSubmissionResponse{
			Success: false,
			Message: "Failed to submit job: " + err.Error(),
		})
		return
	}

	log.Printf("Job submitted via API: %s", jobReq.CodeUri)

	// Return success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(JobSubmissionResponse{
		Success: true,
		Message: "Job submitted successfully",
		JobId:   fmt.Sprintf("job-%d", time.Now().UnixNano()),
	})
}

// handleFileUpload handles REST API file upload requests
func (c *Client) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	// Parse multipart form (no size limit)
	err := r.ParseMultipartForm(0)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "Failed to parse form data: " + err.Error(),
		})
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "No file provided: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Get the upload path from form
	uploadPath := r.FormValue("path")
	if uploadPath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "Upload path is required",
		})
		return
	}

	// Clean the path and ensure it starts with /
	if !strings.HasPrefix(uploadPath, "/") {
		uploadPath = "/" + uploadPath
	}

	// Get volume path from environment
	volumePath := os.Getenv("VOLUME_PATH")
	if volumePath == "" {
		volumePath = "./volume" // Default fallback
	}

	// Construct full file path
	fullPath := filepath.Join(volumePath, uploadPath)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "Failed to create directory: " + err.Error(),
		})
		return
	}

	// Create the destination file
	dst, err := os.Create(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "Failed to create file: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	// Copy file contents
	_, err = io.Copy(dst, file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FileUploadResponse{
			Success: false,
			Message: "Failed to save file: " + err.Error(),
		})
		return
	}

	log.Printf("File uploaded successfully: %s (original: %s)", fullPath, handler.Filename)

	// Return success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(FileUploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		Path:    uploadPath,
	})
}

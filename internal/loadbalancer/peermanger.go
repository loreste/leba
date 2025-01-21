package loadbalancer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Peer represents a peer node in the cluster
type Peer struct {
	NodeID   string
	Address  string
	wsConn   *websocket.Conn
	isActive bool
	mu       sync.RWMutex // Protects wsConn and isActive
}

// PeerManager manages communication with peers in the cluster
type PeerManager struct {
	peers       map[string]*Peer
	backendPool *BackendPool
	mu          sync.RWMutex
	upgrader    websocket.Upgrader
}

// NewPeerManager creates a new PeerManager instance
func NewPeerManager(peerAddresses []string, backendPool *BackendPool) *PeerManager {
	log.Println("Initializing PeerManager...")

	peers := make(map[string]*Peer)
	for _, address := range peerAddresses {
		peers[address] = &Peer{
			Address:  address,
			isActive: false,
		}
	}

	return &PeerManager{
		peers:       peers,
		backendPool: backendPool,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all connections; adjust for security
			},
		},
	}
}

// Start starts the PeerManager to listen for incoming WebSocket connections
func (pm *PeerManager) Start(peerPort string) {
	http.HandleFunc("/ws", pm.handleWebSocket)

	go func() {
		log.Printf("Starting PeerManager server on port %s...", peerPort)
		if err := http.ListenAndServe(":"+peerPort, nil); err != nil {
			log.Fatalf("Failed to start PeerManager server: %v", err)
		}
	}()
}

// handleWebSocket handles incoming WebSocket connections from peers
func (pm *PeerManager) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := pm.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	peerAddress := r.RemoteAddr
	log.Printf("New peer connected: %s", peerAddress)

	peer := &Peer{
		Address:  peerAddress,
		wsConn:   conn,
		isActive: true,
	}

	pm.mu.Lock()
	pm.peers[peerAddress] = peer
	pm.mu.Unlock()

	go pm.handlePeerConnection(peer)
}

// handlePeerConnection manages communication with a connected peer
func (pm *PeerManager) handlePeerConnection(peer *Peer) {
	defer func() {
		peer.wsConn.Close()
		pm.removePeer(peer.Address)
	}()

	for {
		_, message, err := peer.wsConn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from peer %s: %v", peer.Address, err)
			return
		}

		pm.handleMessage(peer, message)
	}
}

// handleMessage processes messages received from peers
func (pm *PeerManager) handleMessage(peer *Peer, message []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(message, &payload); err != nil {
		log.Printf("Invalid message from peer %s: %v", peer.Address, err)
		return
	}

	// Handle different message types
	switch payload["type"] {
	case "state_sync":
		pm.handleStateSync(payload["payload"])
	case "backend_update":
		pm.handleBackendUpdate(payload["payload"])
	default:
		log.Printf("Unknown message type from peer %s: %v", peer.Address, payload["type"])
	}
}

// handleStateSync handles state synchronization messages
func (pm *PeerManager) handleStateSync(data interface{}) {
	var state map[string]*Backend
	if err := mapToStruct(data, &state); err != nil {
		log.Printf("Failed to parse state sync data: %v", err)
		return
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for address, backend := range state {
		if existingBackend, exists := pm.backendPool.backends[address]; exists {
			existingBackend.mu.Lock()
			existingBackend.Health = backend.Health
			existingBackend.ActiveConnections = backend.ActiveConnections
			existingBackend.mu.Unlock()
		} else {
			pm.backendPool.backends[address] = backend
		}
	}

	log.Println("State synchronized successfully.")
}

// handleBackendUpdate handles backend update messages
func (pm *PeerManager) handleBackendUpdate(data interface{}) {
	var backend Backend
	if err := mapToStruct(data, &backend); err != nil {
		log.Printf("Failed to parse backend update data: %v", err)
		return
	}

	pm.backendPool.AddOrUpdateBackend(&backend)
	log.Printf("Backend updated: %v", backend)
}

// BroadcastState sends the local backend state to all connected peers
func (pm *PeerManager) BroadcastState() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	state := make(map[string]*Backend)
	for address, backend := range pm.backendPool.backends {
		state[address] = backend
	}

	message, err := json.Marshal(map[string]interface{}{
		"type":    "state_sync",
		"payload": state,
	})
	if err != nil {
		log.Printf("Failed to serialize state: %v", err)
		return
	}

	for _, peer := range pm.peers {
		if peer.isActive {
			go func(p *Peer) {
				if err := pm.sendMessage(p, message); err != nil {
					log.Printf("Failed to send state to peer %s: %v", p.Address, err)
				}
			}(peer)
		}
	}
}

// sendMessage sends a message to a specific peer
func (pm *PeerManager) sendMessage(peer *Peer, message []byte) error {
	peer.mu.RLock()
	defer peer.mu.RUnlock()

	if peer.wsConn == nil {
		return fmt.Errorf("peer connection is nil: %s", peer.Address)
	}

	return peer.wsConn.WriteMessage(websocket.TextMessage, message)
}

// removePeer removes a peer from the manager
func (pm *PeerManager) removePeer(address string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.peers, address)
	log.Printf("Peer removed: %s", address)
}

// mapToStruct converts a map to a struct using JSON marshalling/unmarshalling
func mapToStruct(data interface{}, target interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}

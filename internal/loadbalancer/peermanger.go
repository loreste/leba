package loadbalancer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"leba/internal/config"

	"github.com/gorilla/websocket"
)

// Peer represents a peer node in the cluster
type Peer struct {
	NodeID   string
	Address  string
	wsConn   *websocket.Conn
	isActive bool
	mu       sync.Mutex
}

// PeeringManager manages communication with peers
type PeeringManager struct {
	peers    map[string]*Peer
	selfNode *Peer
	config   *config.Config
	lb       *LoadBalancer
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

// NewPeeringManager creates a new PeeringManager
func NewPeeringManager(config *config.Config, lb *LoadBalancer) *PeeringManager {
	selfNode := &Peer{
		NodeID:  config.NodeID,
		Address: config.FrontendAddress,
	}
	return &PeeringManager{
		peers:    make(map[string]*Peer),
		selfNode: selfNode,
		config:   config,
		lb:       lb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all connections, implement security measures as needed
			},
		},
	}
}

// Start starts the peering manager and connects to initial peers
func (pm *PeeringManager) Start() {
	for _, peerConfig := range pm.config.InitialPeers {
		go pm.connectToPeer(peerConfig)
	}
	http.HandleFunc("/ws", pm.handleWebSocket)
	log.Printf("Peering server started on %s", pm.config.PeerPort)
	go http.ListenAndServe(":"+pm.config.PeerPort, nil)
}

// connectToPeer attempts to establish a WebSocket connection to a peer
func (pm *PeeringManager) connectToPeer(peerConfig config.PeerConfig) {
	for {
		wsURL := "ws://" + peerConfig.Address + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("Failed to connect to peer %s: %v", peerConfig.NodeID, err)
			continue
		}

		peer := &Peer{
			NodeID:   peerConfig.NodeID,
			Address:  peerConfig.Address,
			wsConn:   conn,
			isActive: true,
		}

		pm.mu.Lock()
		pm.peers[peerConfig.NodeID] = peer
		pm.mu.Unlock()

		log.Printf("Connected to peer: %s", peerConfig.NodeID)

		go pm.handlePeerConnection(peer)

		break
	}
}

// handleWebSocket handles incoming WebSocket connections from peers
func (pm *PeeringManager) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := pm.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	peerID := r.URL.Query().Get("node_id")
	if peerID == "" {
		log.Printf("Peer connected without node_id")
		conn.Close()
		return
	}

	peer := &Peer{
		NodeID:   peerID,
		wsConn:   conn,
		isActive: true,
	}

	pm.mu.Lock()
	pm.peers[peerID] = peer
	pm.mu.Unlock()

	log.Printf("Peer connected: %s", peerID)

	go pm.handlePeerConnection(peer)
}

// handlePeerConnection manages communication with a connected peer
func (pm *PeeringManager) handlePeerConnection(peer *Peer) {
	defer func() {
		peer.wsConn.Close()
		pm.removePeer(peer.NodeID)
	}()

	for {
		_, message, err := peer.wsConn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from peer %s: %v", peer.NodeID, err)
			return
		}

		// Handle incoming messages (e.g., state synchronization, backend updates, peer updates)
		pm.handleIncomingMessage(peer, message)
	}
}

// handleIncomingMessage processes messages received from peers
func (pm *PeeringManager) handleIncomingMessage(peer *Peer, message []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		log.Printf("Failed to unmarshal message from peer %s: %v", peer.NodeID, err)
		return
	}

	// Handle different types of messages
	switch data["type"] {
	case "state_sync":
		pm.handleStateSync(data["payload"])
	case "backend_update":
		pm.handleBackendUpdate(data["payload"])
	case "peer_update":
		pm.handlePeerUpdate(data["payload"])
	default:
		log.Printf("Unknown message type from peer %s: %v", peer.NodeID, data)
	}
}

// handleStateSync handles state synchronization messages
func (pm *PeeringManager) handleStateSync(payload interface{}) {
	var stateData StateData
	if err := mapToStruct(payload, &stateData); err != nil {
		log.Printf("Failed to process state sync data: %v", err)
		return
	}

	// Process state synchronization data
	log.Printf("Received state sync data: %+v", stateData)
	pm.syncState(stateData)

	// Broadcast state sync to all peers
	message, _ := json.Marshal(map[string]interface{}{
		"type":    "state_sync",
		"payload": stateData,
	})
	pm.broadcastToPeers(message)
}

// handleBackendUpdate handles backend update messages
func (pm *PeeringManager) handleBackendUpdate(payload interface{}) {
	var backend *Backend
	if err := mapToStruct(payload, &backend); err != nil {
		log.Printf("Failed to process backend update: %v", err)
		return
	}

	// Update backend details, e.g., health status or active connections
	log.Printf("Received backend update: %+v", backend)
	pm.updateBackend(backend)

	// Notify peers about the backend update
	message, _ := json.Marshal(map[string]interface{}{
		"type":    "backend_update",
		"payload": backend,
	})
	pm.broadcastToPeers(message)
}

// handlePeerUpdate handles peer update messages
func (pm *PeeringManager) handlePeerUpdate(payload interface{}) {
	var peer *Peer
	if err := mapToStruct(payload, &peer); err != nil {
		log.Printf("Failed to process peer update: %v", err)
		return
	}

	// Update peer details, e.g., connection status or address
	log.Printf("Received peer update: %+v", peer)
	pm.updatePeer(peer)

	// Notify the specific peer about the update
	message, _ := json.Marshal(map[string]interface{}{
		"type":    "peer_update",
		"payload": peer,
	})
	if err := pm.sendMessageToPeer(peer.NodeID, message); err != nil {
		log.Printf("Failed to send peer update to %s: %v", peer.NodeID, err)
	}
}

// removePeer removes a peer from the peering manager
func (pm *PeeringManager) removePeer(nodeID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.peers, nodeID)
	log.Printf("Peer removed: %s", nodeID)
}

// broadcastToPeers sends a message to all connected peers
func (pm *PeeringManager) broadcastToPeers(message []byte) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, peer := range pm.peers {
		if peer.isActive {
			if err := peer.wsConn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Failed to send message to peer %s: %v", peer.NodeID, err)
				peer.isActive = false
			}
		}
	}
}

// sendMessageToPeer sends a message to a specific peer
func (pm *PeeringManager) sendMessageToPeer(nodeID string, message []byte) error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	peer, exists := pm.peers[nodeID]
	if !exists || !peer.isActive {
		return fmt.Errorf("peer not available: %s", nodeID)
	}
	return peer.wsConn.WriteMessage(websocket.TextMessage, message)
}

// syncState synchronizes the local state with the state received from a peer
func (pm *PeeringManager) syncState(stateData StateData) {
	pm.lb.backendPool.mu.Lock()
	defer pm.lb.backendPool.mu.Unlock()

	for addr, backend := range stateData.Backends {
		if existingBackend, exists := pm.lb.backendPool.backends[addr]; exists {
			existingBackend.Health = backend.Health
			existingBackend.ActiveConnections = backend.ActiveConnections
		} else {
			pm.lb.backendPool.backends[addr] = backend
		}
	}

	// Update peers if needed, similar logic can be added
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for nodeID, peer := range stateData.Peers {
		if existingPeer, exists := pm.peers[nodeID]; exists {
			existingPeer.Address = peer.Address
			existingPeer.isActive = peer.isActive
		} else {
			pm.peers[nodeID] = peer
		}
	}
}

// updateBackend updates a single backend's information
func (pm *PeeringManager) updateBackend(backend *Backend) {
	pm.lb.backendPool.mu.Lock()
	defer pm.lb.backendPool.mu.Unlock()

	if existingBackend, exists := pm.lb.backendPool.backends[backend.Address]; exists {
		existingBackend.Health = backend.Health
		existingBackend.ActiveConnections = backend.ActiveConnections
		existingBackend.Weight = backend.Weight
	} else {
		pm.lb.backendPool.backends[backend.Address] = backend
	}
}

// updatePeer updates a peer's information
func (pm *PeeringManager) updatePeer(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if existingPeer, exists := pm.peers[peer.NodeID]; exists {
		existingPeer.Address = peer.Address
		existingPeer.isActive = peer.isActive
		if existingPeer.wsConn != nil && !existingPeer.isActive {
			existingPeer.wsConn.Close()
			existingPeer.wsConn = nil
		}
	} else {
		pm.peers[peer.NodeID] = peer
	}
}

// StateData represents the state information exchanged between peers
type StateData struct {
	Backends map[string]*Backend `json:"backends"`
	Peers    map[string]*Peer    `json:"peers"`
}

// mapToStruct converts a map to a struct using JSON marshalling/unmarshalling
func mapToStruct(data interface{}, target interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}

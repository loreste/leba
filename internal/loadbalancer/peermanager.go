package loadbalancer

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"leba/internal/shared"

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

// PeeringManager manages communication with peers
type PeeringManager struct {
	peers    map[string]*Peer
	selfNode *Peer
	config   shared.Config
	lb       *LoadBalancer
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

// NewPeeringManager creates a new PeeringManager
func NewPeeringManager(config shared.Config, lb *LoadBalancer) *PeeringManager {
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
				return true // Allow all connections; implement security measures as needed
			},
		},
	}
}

// SetLoadBalancer assigns a LoadBalancer instance to the PeeringManager
func (pm *PeeringManager) SetLoadBalancer(lb *LoadBalancer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.lb = lb
	log.Println("LoadBalancer instance assigned to PeeringManager")
}

// Start initializes the peering manager and connects to initial peers
func (pm *PeeringManager) Start() {
	for _, peerConfig := range pm.config.InitialPeers {
		go pm.connectToPeer(peerConfig)
	}
	http.HandleFunc("/ws", pm.handleWebSocket)
	log.Printf("Peering server started on %s", pm.config.PeerPort)
	go func() {
		if err := http.ListenAndServe(":"+pm.config.PeerPort, nil); err != nil {
			log.Fatalf("Failed to start peering server: %v", err)
		}
	}()
}

// connectToPeer establishes a WebSocket connection to a peer
func (pm *PeeringManager) connectToPeer(peerConfig shared.PeerConfig) {
	for {
		wsURL := "ws://" + peerConfig.Address + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("Failed to connect to peer %s: %v", peerConfig.NodeID, err)
			time.Sleep(5 * time.Second) // Retry delay
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

		log.Printf("Connected to peer: %s at %s", peerConfig.NodeID, peerConfig.Address)

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

	log.Printf("Received state sync data: %+v", stateData)
	pm.syncState(stateData)
}

// handleBackendUpdate handles backend update messages
func (pm *PeeringManager) handleBackendUpdate(payload interface{}) {
	var backend *Backend
	if err := mapToStruct(payload, &backend); err != nil {
		log.Printf("Failed to process backend update: %v", err)
		return
	}

	log.Printf("Received backend update: %+v", backend)
	pm.lb.backendPool.AddOrUpdateBackend(backend)
}

// handlePeerUpdate handles peer update messages
func (pm *PeeringManager) handlePeerUpdate(payload interface{}) {
	var peer *Peer
	if err := mapToStruct(payload, &peer); err != nil {
		log.Printf("Failed to process peer update: %v", err)
		return
	}

	log.Printf("Received peer update: %+v", peer)
	pm.updatePeer(peer)
}

// removePeer removes a peer from the peering manager
func (pm *PeeringManager) removePeer(nodeID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.peers, nodeID)
	log.Printf("Peer removed: %s", nodeID)
}

// syncState synchronizes the local state with the state received from a peer
func (pm *PeeringManager) syncState(stateData StateData) {
	pm.lb.backendPool.mu.Lock()
	defer pm.lb.backendPool.mu.Unlock()

	for _, backend := range stateData.Backends {
		pm.lb.backendPool.AddOrUpdateBackend(backend)
	}

	log.Printf("State synchronized with peer")
}

// updatePeer updates a peer's information
func (pm *PeeringManager) updatePeer(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	existingPeer, exists := pm.peers[peer.NodeID]
	if exists {
		existingPeer.mu.Lock()
		existingPeer.Address = peer.Address
		existingPeer.isActive = peer.isActive
		existingPeer.mu.Unlock()
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

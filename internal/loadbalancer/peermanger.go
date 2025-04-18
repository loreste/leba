package loadbalancer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// PeerManager represents a manager for peer nodes in the cluster
type PeerManager struct {
	peerAddresses []string
	backendPool   *BackendPool
	mu            sync.RWMutex
}

// NewPeerManager creates a new PeerManager
func NewPeerManager(peerAddresses []string, backendPool *BackendPool) *PeerManager {
	return &PeerManager{
		peerAddresses: peerAddresses,
		backendPool:   backendPool,
	}
}

// BroadcastState sends the current state to all peers
func (pm *PeerManager) BroadcastState() {
	pm.mu.RLock()
	peers := make([]string, len(pm.peerAddresses))
	copy(peers, pm.peerAddresses)
	pm.mu.RUnlock()

	for _, peer := range peers {
		go pm.sendStateToPeer(peer)
	}
}

// sendStateToPeer sends the backend state to a specific peer
func (pm *PeerManager) sendStateToPeer(peerAddress string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get current backends
	pm.backendPool.mu.RLock()
	backends := pm.backendPool.ListBackends()
	pm.backendPool.mu.RUnlock()

	// Marshal backend data
	backendData, err := json.Marshal(backends)
	if err != nil {
		log.Printf("Failed to marshal backend data: %v", err)
		return
	}

	// Prepare HTTP request
	url := fmt.Sprintf("http://%s/update-state", peerAddress)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(backendData))
	if err != nil {
		log.Printf("Failed to create request for peer %s: %v", peerAddress, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send state to peer %s: %v", peerAddress, err)
		return
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		log.Printf("Peer %s returned non-OK status: %d", peerAddress, resp.StatusCode)
	} else {
		log.Printf("State successfully sent to peer %s", peerAddress)
	}
}

// AddPeer adds a new peer to the peer list
func (pm *PeerManager) AddPeer(address string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if peer already exists
	for _, peer := range pm.peerAddresses {
		if peer == address {
			return // Already exists
		}
	}

	pm.peerAddresses = append(pm.peerAddresses, address)
	log.Printf("Added peer: %s", address)

	// Send current state to the new peer
	go pm.sendStateToPeer(address)
}

// RemovePeer removes a peer from the peer list
func (pm *PeerManager) RemovePeer(address string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i, peer := range pm.peerAddresses {
		if peer == address {
			// Remove peer by replacing with last element and truncating
			pm.peerAddresses[i] = pm.peerAddresses[len(pm.peerAddresses)-1]
			pm.peerAddresses = pm.peerAddresses[:len(pm.peerAddresses)-1]
			log.Printf("Removed peer: %s", address)
			return
		}
	}
}

// UpdateState processes state updates received from peers
func (pm *PeerManager) UpdateState(backendData []byte) error {
	var backends []*Backend
	if err := json.Unmarshal(backendData, &backends); err != nil {
		return fmt.Errorf("failed to unmarshal backend data: %v", err)
	}

	pm.backendPool.mu.Lock()
	defer pm.backendPool.mu.Unlock()

	// Update existing backends or add new ones
	for _, backend := range backends {
		existingBackend, exists := pm.backendPool.backends[backend.Address]
		if exists {
			// Update existing backend
			existingBackend.mu.Lock()
			existingBackend.Health = backend.Health
			existingBackend.ActiveConnections = backend.ActiveConnections
			existingBackend.mu.Unlock()
		} else {
			// Add new backend
			pm.backendPool.backends[backend.Address] = backend
		}
	}

	log.Printf("Updated state with data from peer (%d backends)", len(backends))
	return nil
}

// SetupHTTPHandlers sets up the HTTP handlers for peer communication
func (pm *PeerManager) SetupHTTPHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/update-state", pm.handleStateUpdate)
	mux.HandleFunc("/peers", pm.handlePeers)
}

// handleStateUpdate handles incoming state updates from peers
func (pm *PeerManager) handleStateUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if err := pm.UpdateState(body); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update state: %v", err), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "State updated successfully")
}

// handlePeers handles adding and removing peers
func (pm *PeerManager) handlePeers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Add a new peer
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var peerRequest struct {
			Address string `json:"address"`
		}
		if err := json.Unmarshal(body, &peerRequest); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if peerRequest.Address == "" {
			http.Error(w, "Address is required", http.StatusBadRequest)
			return
		}

		pm.AddPeer(peerRequest.Address)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Peer added successfully")

	case http.MethodDelete:
		// Remove a peer
		peerAddress := r.URL.Query().Get("address")
		if peerAddress == "" {
			http.Error(w, "Address query parameter is required", http.StatusBadRequest)
			return
		}

		pm.RemovePeer(peerAddress)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Peer removed successfully")

	case http.MethodGet:
		// List all peers
		pm.mu.RLock()
		peers := make([]string, len(pm.peerAddresses))
		copy(peers, pm.peerAddresses)
		pm.mu.RUnlock()

		response, err := json.Marshal(peers)
		if err != nil {
			http.Error(w, "Failed to marshal peer list", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
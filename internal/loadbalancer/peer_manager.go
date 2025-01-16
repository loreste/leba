package loadbalancer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// PeerConfig represents the configuration for a peer node.
type PeerConfig struct {
	Address string `yaml:"address"`
	NodeID  string `yaml:"node_id"`
}

// PeerManager manages communication and synchronization between nodes in the cluster.
type PeerManager struct {
	peers        []PeerConfig
	stateMutex   sync.RWMutex
	backendPool  *BackendPool
	syncInterval time.Duration
}

// NewPeerManager creates a new PeerManager instance.
func NewPeerManager(peers []PeerConfig, backendPool *BackendPool, syncInterval time.Duration) *PeerManager {
	log.Println("Initializing PeerManager...")
	return &PeerManager{
		peers:        peers,
		backendPool:  backendPool,
		syncInterval: syncInterval,
	}
}

// SyncPeers periodically synchronizes the peer state.
func (pm *PeerManager) SyncPeers() error {
	log.Println("Starting peer synchronization...")
	ticker := time.NewTicker(pm.syncInterval)
	defer ticker.Stop()

	for range ticker.C {
		pm.stateMutex.RLock()
		for _, peer := range pm.peers {
			go func(peer PeerConfig) {
				if err := pm.sendStateToPeer(peer); err != nil {
					log.Printf("Error syncing state to peer %s: %v", peer.Address, err)
				}
			}(peer)
		}
		pm.stateMutex.RUnlock()
	}
	return nil
}

// BroadcastState sends the local backend pool state to all peers.
func (pm *PeerManager) BroadcastState() {
	pm.stateMutex.RLock()
	defer pm.stateMutex.RUnlock()

	for _, peer := range pm.peers {
		go func(peer PeerConfig) {
			if err := pm.sendStateToPeer(peer); err != nil {
				log.Printf("Failed to send state to peer %s: %v", peer.Address, err)
			}
		}(peer)
	}
}

// sendStateToPeer sends the backend pool state to a specific peer.
func (pm *PeerManager) sendStateToPeer(peer PeerConfig) error {
	pm.stateMutex.RLock()
	defer pm.stateMutex.RUnlock()

	stateData, err := json.Marshal(pm.backendPool.ListBackends())
	if err != nil {
		return fmt.Errorf("failed to marshal backend pool state: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/update_state", peer.Address), bytes.NewBuffer(stateData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to peer %s: %w", peer.Address, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-OK response from peer %s: %s", peer.Address, resp.Status)
	}

	log.Printf("Successfully synced state to peer %s", peer.Address)
	return nil
}

// UpdateState updates the local backend pool state with data from a peer.
func (pm *PeerManager) UpdateState(newState []Backend) {
	pm.stateMutex.Lock()
	defer pm.stateMutex.Unlock()

	for i := range newState {
		pm.backendPool.AddOrUpdateBackend(&newState[i])
	}
	log.Println("Updated backend pool state with data from peer.")
}

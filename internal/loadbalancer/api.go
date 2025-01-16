package loadbalancer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// APIHandler is responsible for handling API-related requests.
type APIHandler struct {
	backendPool *BackendPool
}

// NewAPIHandler creates a new APIHandler with the provided BackendPool.
func NewAPIHandler(backendPool *BackendPool) *APIHandler {
	return &APIHandler{
		backendPool: backendPool,
	}
}

// StartAPI starts the API server for managing and monitoring the load balancer.
func StartAPI(port string, backendPool *BackendPool) error {
	apiHandler := NewAPIHandler(backendPool)

	http.HandleFunc("/backends", apiHandler.handleBackends)
	http.HandleFunc("/health", apiHandler.handleHealth)

	address := fmt.Sprintf(":%s", port)
	log.Printf("API server is running on %s", address)
	return http.ListenAndServe(address, nil)
}

// handleBackends handles the /backends endpoint.
func (api *APIHandler) handleBackends(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listBackends(w)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listBackends returns a list of all backends in the pool.
func (api *APIHandler) listBackends(w http.ResponseWriter) {
	backends := api.backendPool.ListBackends()
	response, err := json.Marshal(backends)
	if err != nil {
		http.Error(w, "Failed to serialize backends", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// handleHealth handles the /health endpoint.
func (api *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

package loadbalancer

import (
	"encoding/json"
	"net/http"
)

// Handlers struct encapsulates dependencies for HTTP handlers
type Handlers struct {
	BackendPool *BackendPool
}

// NewHandlers creates a new instance of Handlers
func NewHandlers(pool *BackendPool) *Handlers {
	return &Handlers{BackendPool: pool}
}

// AddBackendHandler handles adding a new backend
func (h *Handlers) AddBackendHandler(w http.ResponseWriter, r *http.Request) {
	var backend Backend
	if err := json.NewDecoder(r.Body).Decode(&backend); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	h.BackendPool.AddBackend(&backend)
	w.WriteHeader(http.StatusCreated)
}

// RemoveBackendHandler handles removing an existing backend
func (h *Handlers) RemoveBackendHandler(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Address query parameter is required", http.StatusBadRequest)
		return
	}

	h.BackendPool.RemoveBackend(address)
	w.WriteHeader(http.StatusNoContent)
}

// ListBackendsHandler handles listing all backends
func (h *Handlers) ListBackendsHandler(w http.ResponseWriter, r *http.Request) {
	backends := h.BackendPool.ListBackends()
	if err := json.NewEncoder(w).Encode(backends); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// RegisterRoutes registers HTTP routes and handlers
func (h *Handlers) RegisterRoutes() {
	http.HandleFunc("/backends", h.ListBackendsHandler)
	http.HandleFunc("/backends/add", h.AddBackendHandler)
	http.HandleFunc("/backends/remove", h.RemoveBackendHandler)
}

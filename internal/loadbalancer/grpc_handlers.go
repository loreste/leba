package loadbalancer

import (
	"context"
	"fmt"
	"log"

	pb "leba/internal/loadbalancer/proto" // Correct import for generated protobuf code

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPCServer implements the LoadBalancer gRPC server interface
type GRPCServer struct {
	pb.UnimplementedLoadBalancerServer // Embedding for forward compatibility
	BackendPool                        *BackendPool
}

// NewGRPCServer creates a new GRPCServer instance
func NewGRPCServer(pool *BackendPool) *GRPCServer {
	return &GRPCServer{BackendPool: pool}
}

// AddBackend handles the gRPC request to add a backend server
func (s *GRPCServer) AddBackend(ctx context.Context, req *pb.BackendRequest) (*pb.BackendResponse, error) {
	backend := &Backend{
		Address:            req.Address,
		Protocol:           req.Protocol,
		Port:               int(req.Port),
		Weight:             int(req.Weight),
		MaxOpenConnections: int(req.MaxOpenConnections),
		MaxIdleConnections: int(req.MaxIdleConnections),
		ConnMaxLifetime:    int(req.ConnMaxLifetime),
		Health:             true, // Assume healthy upon addition
	}
	s.BackendPool.AddBackend(backend)
	log.Printf("Added backend: %v", backend)
	return &pb.BackendResponse{Message: "Backend added successfully"}, nil
}

// RemoveBackend handles the gRPC request to remove a backend server
func (s *GRPCServer) RemoveBackend(ctx context.Context, req *pb.RemoveBackendRequest) (*pb.BackendResponse, error) {
	address := req.Address
	if address == "" {
		return nil, fmt.Errorf("address is required")
	}

	s.BackendPool.RemoveBackend(address)
	log.Printf("Removed backend: %s", address)
	return &pb.BackendResponse{Message: "Backend removed successfully"}, nil
}

// ListBackends handles the gRPC request to list all backend servers
func (s *GRPCServer) ListBackends(ctx context.Context, req *pb.ListBackendsRequest) (*pb.ListBackendsResponse, error) {
	backends := s.BackendPool.ListBackends()
	var grpcBackends []*pb.BackendInfo
	for _, backend := range backends {
		grpcBackends = append(grpcBackends, &pb.BackendInfo{
			Address:            backend.Address,
			Protocol:           backend.Protocol,
			Port:               int32(backend.Port),
			Weight:             int32(backend.Weight),
			MaxOpenConnections: int32(backend.MaxOpenConnections),
			MaxIdleConnections: int32(backend.MaxIdleConnections),
			ConnMaxLifetime:    int32(backend.ConnMaxLifetime),
			Health:             backend.Health,
		})
	}
	return &pb.ListBackendsResponse{Backends: grpcBackends}, nil
}

// RegisterGRPCServer registers the gRPC server with the gRPC server instance
func RegisterGRPCServer(s *grpc.Server, pool *BackendPool) {
	grpcServer := NewGRPCServer(pool)
	pb.RegisterLoadBalancerServer(s, grpcServer)
	reflection.Register(s) // Enable reflection for easier debugging and introspection
}

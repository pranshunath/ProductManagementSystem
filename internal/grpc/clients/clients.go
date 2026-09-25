package clients

import (
	"fmt"
	"log"

	"producthub/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClients bundles client stubs for communicating with internal gRPC services
type GRPCClients struct {
	Conn            *grpc.ClientConn
	ProductClient   pb.ProductServiceClient
	InventoryClient pb.InventoryServiceClient
	OrderClient     pb.OrderServiceClient
}

// NewGRPCClients establishes an internal gRPC client connection to the specified target address
func NewGRPCClients(targetAddr string) (*GRPCClients, error) {
	conn, err := grpc.NewClient(targetAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client connection to %s: %w", targetAddr, err)
	}

	log.Printf("[GRPC CLIENT] Connected to internal gRPC mesh at %s\n", targetAddr)

	return &GRPCClients{
		Conn:            conn,
		ProductClient:   pb.NewProductServiceClient(conn),
		InventoryClient: pb.NewInventoryServiceClient(conn),
		OrderClient:     pb.NewOrderServiceClient(conn),
	}, nil
}

// Close gracefully terminates the gRPC client connection
func (c *GRPCClients) Close() error {
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

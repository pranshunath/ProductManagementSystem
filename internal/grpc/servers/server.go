package servers

import (
	"fmt"
	"log"
	"net"

	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// InitGRPCServer creates and configures a new gRPC server with registered service handlers
func InitGRPCServer(
	prodService services.ProductService,
	invService services.InventoryService,
	orderService services.OrderService,
	invRepo repositories.InventoryRepository,
) *grpc.Server {
	grpcServer := grpc.NewServer()

	// Register Product Service
	if prodService != nil {
		prodGrpc := NewProductGrpcServer(prodService)
		pb.RegisterProductServiceServer(grpcServer, prodGrpc)
	}

	// Register Inventory Service
	if invService != nil && invRepo != nil {
		invGrpc := NewInventoryGrpcServer(invService, invRepo)
		pb.RegisterInventoryServiceServer(grpcServer, invGrpc)
	}

	// Register Order Service
	if orderService != nil {
		orderGrpc := NewOrderGrpcServer(orderService)
		pb.RegisterOrderServiceServer(grpcServer, orderGrpc)
	}

	// Enable gRPC Reflection for debugging with tools like Postman or grpcurl
	reflection.Register(grpcServer)

	return grpcServer
}

// StartGRPCServer binds a TCP port and begins listening for gRPC requests
func StartGRPCServer(port string, server *grpc.Server) (net.Listener, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return nil, fmt.Errorf("failed to bind gRPC TCP port %s: %w", port, err)
	}

	log.Printf("[GRPC] gRPC Server listening on port :%s\n", port)
	go func() {
		if err := server.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("[ERROR] gRPC server stopped unexpectedly: %v\n", err)
		}
	}()

	return lis, nil
}

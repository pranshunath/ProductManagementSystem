package tests

import (
	"context"
	"net"
	"testing"
	"time"

	"producthub/internal/grpc/clients"
	"producthub/internal/grpc/servers"
	"producthub/internal/services"
	"producthub/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestGRPC_ProductAndInventoryService(t *testing.T) {
	catRepo := NewMockCategoryRepo()
	prodRepo := NewMockProductRepo()
	invRepo := &MockInventoryRepo{}

	catService := services.NewCategoryService(catRepo)
	prodService := services.NewProductService(prodRepo, catRepo, invRepo)
	invService := services.NewInventoryService(invRepo, prodRepo)

	// Seed test data
	cat, err := catService.CreateCategory("Hardware", "PC Parts")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	prod, err := prodService.CreateProduct("GPU-RTX-4090", "GeForce RTX 4090 24GB", "Flagship gaming GPU", cat.ID, 1599.99, 10)
	if err != nil {
		t.Fatalf("Failed to create product: %v", err)
	}

	// 1. Start gRPC server on arbitrary free loopback port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on loopback: %v", err)
	}
	defer lis.Close()

	grpcServer := servers.InitGRPCServer(prodService, invService, nil, invRepo)
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.GracefulStop()

	// 2. Establish gRPC client connection
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	prodClient := pb.NewProductServiceClient(conn)
	invClient := pb.NewInventoryServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 3. Test RPC: GetProduct by ID
	getResp, err := prodClient.GetProduct(ctx, &pb.GetProductRequest{Id: uint32(prod.ID)})
	if err != nil {
		t.Fatalf("GetProduct RPC failed: %v", err)
	}

	if getResp.Product == nil || getResp.Product.Sku != "GPU-RTX-4090" {
		t.Errorf("Expected SKU GPU-RTX-4090, got %v", getResp.Product)
	}
	if getResp.Product.AvailableStock != 10 {
		t.Errorf("Expected available stock 10, got %d", getResp.Product.AvailableStock)
	}

	// 4. Test RPC: GetProduct by SKU
	getBySkuResp, err := prodClient.GetProduct(ctx, &pb.GetProductRequest{Sku: "GPU-RTX-4090"})
	if err != nil {
		t.Fatalf("GetProduct by SKU RPC failed: %v", err)
	}
	if getBySkuResp.Product.Id != uint32(prod.ID) {
		t.Errorf("Expected ID %d, got %d", prod.ID, getBySkuResp.Product.Id)
	}

	// 5. Test RPC: GetProduct Not Found
	_, errNotFound := prodClient.GetProduct(ctx, &pb.GetProductRequest{Id: 99999})
	if errNotFound == nil {
		t.Errorf("Expected error for non-existent product")
	} else {
		st, ok := status.FromError(errNotFound)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound gRPC code, got %v", errNotFound)
		}
	}

	// 6. Test RPC: ListProducts
	listResp, err := prodClient.ListProducts(ctx, &pb.ListProductsRequest{
		Page:   1,
		Limit:  10,
		Search: "RTX",
	})
	if err != nil {
		t.Fatalf("ListProducts RPC failed: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Products) != 1 {
		t.Errorf("Expected 1 product in list, got total=%d, count=%d", listResp.Total, len(listResp.Products))
	}

	// 7. Test RPC: CheckStock
	stockResp, err := invClient.CheckStock(ctx, &pb.CheckStockRequest{
		ProductId: uint32(prod.ID),
		Quantity:  5,
	})
	if err != nil {
		t.Fatalf("CheckStock RPC failed: %v", err)
	}
	if !stockResp.CanFulfill || stockResp.AvailableStock != 10 {
		t.Errorf("Expected can fulfill with 10 available stock, got %v", stockResp)
	}

	// 8. Test RPC: ReserveStock
	reserveResp, err := invClient.ReserveStock(ctx, &pb.ReserveStockRequest{
		ProductId:   uint32(prod.ID),
		Quantity:    2,
		ReferenceId: "TEST-GRPC-REF-1",
	})
	if err != nil {
		t.Fatalf("ReserveStock RPC failed: %v", err)
	}
	if !reserveResp.Success {
		t.Errorf("Expected ReserveStock to succeed: %v", reserveResp.Message)
	}
}

func TestGRPCClients_Factory(t *testing.T) {
	// Start loopback server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on loopback: %v", err)
	}
	defer lis.Close()

	grpcServer := servers.InitGRPCServer(nil, nil, nil, nil)
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.GracefulStop()

	// Connect using GRPCClients bundle
	grpcClients, err := clients.NewGRPCClients(lis.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create GRPCClients: %v", err)
	}
	defer grpcClients.Close()

	if grpcClients.ProductClient == nil || grpcClients.InventoryClient == nil || grpcClients.OrderClient == nil {
		t.Errorf("Expected all client stubs to be instantiated")
	}
}

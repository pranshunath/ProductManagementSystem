package servers

import (
	"context"

	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/pkg/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InventoryGrpcServer implements pb.InventoryServiceServer
type InventoryGrpcServer struct {
	pb.UnimplementedInventoryServiceServer
	invService services.InventoryService
	invRepo    repositories.InventoryRepository
}

// NewInventoryGrpcServer returns an instance of InventoryGrpcServer
func NewInventoryGrpcServer(invService services.InventoryService, invRepo repositories.InventoryRepository) *InventoryGrpcServer {
	return &InventoryGrpcServer{
		invService: invService,
		invRepo:    invRepo,
	}
}

func (s *InventoryGrpcServer) CheckStock(ctx context.Context, req *pb.CheckStockRequest) (*pb.CheckStockResponse, error) {
	if req.ProductId == 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id is required")
	}

	canFulfill, avail, err := s.invService.CheckStock(uint(req.ProductId), int(req.Quantity))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "check stock failed: %v", err)
	}

	return &pb.CheckStockResponse{
		ProductId:      req.ProductId,
		AvailableStock: int32(avail),
		CanFulfill:     canFulfill,
	}, nil
}

func (s *InventoryGrpcServer) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	if req.ProductId == 0 || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "valid product_id and quantity > 0 required")
	}

	err := s.invRepo.ReserveStockAtomic(nil, uint(req.ProductId), int(req.Quantity), req.ReferenceId)
	if err != nil {
		return &pb.ReserveStockResponse{
			Success: false,
			Message: err.Error(),
		}, status.Errorf(codes.FailedPrecondition, "failed to reserve stock: %v", err)
	}

	return &pb.ReserveStockResponse{
		Success: true,
		Message: "Stock successfully reserved",
	}, nil
}

func (s *InventoryGrpcServer) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest) (*pb.ReleaseStockResponse, error) {
	if req.ProductId == 0 || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "valid product_id and quantity > 0 required")
	}

	err := s.invRepo.ReleaseStockAtomic(nil, uint(req.ProductId), int(req.Quantity), req.ReferenceId)
	if err != nil {
		return &pb.ReleaseStockResponse{
			Success: false,
			Message: err.Error(),
		}, status.Errorf(codes.Internal, "failed to release stock: %v", err)
	}

	return &pb.ReleaseStockResponse{
		Success: true,
		Message: "Stock successfully released",
	}, nil
}

func (s *InventoryGrpcServer) Restock(ctx context.Context, req *pb.RestockRequest) (*pb.RestockResponse, error) {
	if req.ProductId == 0 || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "valid product_id and quantity > 0 required")
	}

	product, err := s.invService.RestockProduct(uint(req.ProductId), int(req.Quantity), req.Notes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "restock operation failed: %v", err)
	}

	return &pb.RestockResponse{
		ProductId:         uint32(product.ID),
		NewStock:          int32(product.Stock),
		NewAvailableStock: int32(product.AvailableStock()),
	}, nil
}

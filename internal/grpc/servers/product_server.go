package servers

import (
	"context"
	"time"

	"producthub/internal/models"
	"producthub/internal/repositories"
	"producthub/internal/services"
	"producthub/pkg/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProductGrpcServer implements pb.ProductServiceServer
type ProductGrpcServer struct {
	pb.UnimplementedProductServiceServer
	prodService services.ProductService
}

// NewProductGrpcServer returns an instance of ProductGrpcServer
func NewProductGrpcServer(prodService services.ProductService) *ProductGrpcServer {
	return &ProductGrpcServer{prodService: prodService}
}

func (s *ProductGrpcServer) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	var prod *models.Product
	var err error

	if req.Id > 0 {
		prod, err = s.prodService.GetProductByID(uint(req.Id))
	} else if req.Sku != "" {
		prod, err = s.prodService.GetProductBySKU(req.Sku)
	} else {
		return nil, status.Error(codes.InvalidArgument, "either id or sku must be provided")
	}

	if err != nil {
		return nil, status.Errorf(codes.NotFound, "product not found: %v", err)
	}

	return &pb.GetProductResponse{
		Product: toProductMessage(prod),
	}, nil
}

func (s *ProductGrpcServer) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	filter := repositories.ProductFilter{
		Page:       int(req.Page),
		Limit:      int(req.Limit),
		Search:     req.Search,
		CategoryID: uint(req.CategoryId),
		Category:   req.Category,
		Status:     req.Status,
		Sort:       req.Sort,
		Order:      req.Order,
	}

	if req.MinPrice != nil {
		minP := req.GetMinPrice()
		filter.MinPrice = &minP
	}
	if req.MaxPrice != nil {
		maxP := req.GetMaxPrice()
		filter.MaxPrice = &maxP
	}

	products, pagination, err := s.prodService.ListProducts(filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list products: %v", err)
	}

	messages := make([]*pb.ProductMessage, len(products))
	for i, p := range products {
		messages[i] = &pb.ProductMessage{
			Id:             uint32(p.ID),
			Sku:            p.SKU,
			Name:           p.Name,
			Description:    p.Description,
			ImageUrl:       p.ImageURL,
			CategoryId:     uint32(p.CategoryID),
			CategoryName:   p.CategoryName,
			Price:          p.Price,
			Stock:          int32(p.Stock),
			ReservedStock:  int32(p.ReservedStock),
			AvailableStock: int32(p.AvailableStock),
			StockStatus:    p.StockStatus,
			Status:         string(p.Status),
			CreatedAt:      p.CreatedAt.Format(time.RFC3339),
		}
	}

	return &pb.ListProductsResponse{
		Products:   messages,
		Total:      pagination.Total,
		Page:       int32(pagination.Page),
		Limit:      int32(pagination.Limit),
		TotalPages: int32(pagination.TotalPages),
	}, nil
}

func toProductMessage(p *models.Product) *pb.ProductMessage {
	catName := ""
	if p.Category.Name != "" {
		catName = p.Category.Name
	}

	return &pb.ProductMessage{
		Id:             uint32(p.ID),
		Sku:            p.SKU,
		Name:           p.Name,
		Description:    p.Description,
		ImageUrl:       p.ImageURL,
		CategoryId:     uint32(p.CategoryID),
		CategoryName:   catName,
		Price:          p.Price,
		Stock:          int32(p.Stock),
		ReservedStock:  int32(p.ReservedStock),
		AvailableStock: int32(p.AvailableStock()),
		StockStatus:    p.StockStatus(),
		Status:         string(p.Status),
		CreatedAt:      p.CreatedAt.Format(time.RFC3339),
	}
}

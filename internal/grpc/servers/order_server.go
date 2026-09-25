package servers

import (
	"context"
	"time"

	"producthub/internal/models"
	"producthub/internal/services"
	"producthub/pkg/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrderGrpcServer implements pb.OrderServiceServer
type OrderGrpcServer struct {
	pb.UnimplementedOrderServiceServer
	orderService services.OrderService
}

// NewOrderGrpcServer returns an instance of OrderGrpcServer
func NewOrderGrpcServer(orderService services.OrderService) *OrderGrpcServer {
	return &OrderGrpcServer{orderService: orderService}
}

func (s *OrderGrpcServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	if req.UserId == 0 || len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id and at least one order item required")
	}

	items := make([]services.OrderItemInput, len(req.Items))
	for i, item := range req.Items {
		items[i] = services.OrderItemInput{
			ProductID: uint(item.ProductId),
			Quantity:  int(item.Quantity),
		}
	}

	order, err := s.orderService.CreateOrder(uint(req.UserId), items, req.IdempotencyKey)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to create order: %v", err)
	}

	return &pb.CreateOrderResponse{
		Order: toOrderMessage(order),
	}, nil
}

func (s *OrderGrpcServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "order id is required")
	}

	order, err := s.orderService.GetOrderByID(uint(req.Id), uint(req.UserId), req.IsAdmin)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "order lookup failed: %v", err)
	}

	return &pb.GetOrderResponse{
		Order: toOrderMessage(order),
	}, nil
}

func (s *OrderGrpcServer) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	limit := int(req.Limit)
	if limit < 1 {
		limit = 10
	}

	var orders []models.Order
	var pagination interface{ GetTotal() int64 }
	var total int64
	var totalPages int

	if req.IsAdmin {
		list, pag, err := s.orderService.ListAllOrders(page, limit, models.OrderStatus(req.Status))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list orders: %v", err)
		}
		orders = list
		total = pag.Total
		totalPages = pag.TotalPages
	} else {
		list, pag, err := s.orderService.ListUserOrders(uint(req.UserId), page, limit)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list user orders: %v", err)
		}
		orders = list
		total = pag.Total
		totalPages = pag.TotalPages
	}
	_ = pagination

	orderMessages := make([]*pb.OrderMessage, len(orders))
	for i, o := range orders {
		orderMessages[i] = toOrderMessage(&o)
	}

	return &pb.ListOrdersResponse{
		Orders:     orderMessages,
		Total:      total,
		Page:       int32(page),
		Limit:      int32(limit),
		TotalPages: int32(totalPages),
	}, nil
}

func (s *OrderGrpcServer) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "order id is required")
	}

	order, err := s.orderService.CancelOrder(uint(req.Id), uint(req.UserId), req.IsAdmin)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to cancel order: %v", err)
	}

	return &pb.CancelOrderResponse{
		Order: toOrderMessage(order),
	}, nil
}

func (s *OrderGrpcServer) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest) (*pb.UpdateOrderStatusResponse, error) {
	if req.Id == 0 {
		return nil, status.Error(codes.InvalidArgument, "order id is required")
	}

	order, err := s.orderService.UpdateOrderStatus(uint(req.Id), models.OrderStatus(req.Status))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to update order status: %v", err)
	}

	return &pb.UpdateOrderStatusResponse{
		Order: toOrderMessage(order),
	}, nil
}

func toOrderMessage(o *models.Order) *pb.OrderMessage {
	items := make([]*pb.OrderItemMessage, len(o.Items))
	for i, item := range o.Items {
		items[i] = &pb.OrderItemMessage{
			Id:          uint32(item.ID),
			ProductId:   uint32(item.ProductID),
			ProductName: item.Product.Name,
			ProductSku:  item.Product.SKU,
			Quantity:    int32(item.Quantity),
			UnitPrice:   item.UnitPrice,
			Subtotal:    item.Subtotal,
		}
	}

	userEmail := ""
	if o.User.Email != "" {
		userEmail = o.User.Email
	}

	return &pb.OrderMessage{
		Id:          uint32(o.ID),
		UserId:      uint32(o.UserID),
		UserEmail:   userEmail,
		Status:      string(o.Status),
		TotalAmount: o.TotalAmount,
		Items:       items,
		CreatedAt:   o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   o.UpdatedAt.Format(time.RFC3339),
	}
}

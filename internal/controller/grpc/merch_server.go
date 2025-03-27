package grpc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"merch-store-grpc/api/pb"
	"merch-store-grpc/internal/service"
	"time"
)

type Server struct {
	pb.UnimplementedMerchServiceServer
	svc service.MerchStoreService
}

func NewServer(svc service.MerchStoreService) *Server {
	return &Server{
		svc: svc,
	}
}

func (s *Server) Authenticate(ctx context.Context, req *pb.AuthRequest) (*pb.AuthResponse, error) {
	token, err := s.svc.Authenticate(ctx, req.Username, req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "authentication: %v", err)
	}
	return &pb.AuthResponse{Token: token}, nil
}

func (s *Server) PurchaseMerch(ctx context.Context, req *pb.PurchaseRequest) (*pb.PurchaseResponse, error) {
	userIDVal := ctx.Value("userID")
	if userIDVal == nil {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, status.Error(codes.Internal, "invalid userID in context")
	}

	if err := s.svc.PurchaseMerch(ctx, userID, req.MerchName); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "purchase failed: %v", err)
	}

	return &pb.PurchaseResponse{
		Success: true,
		Message: "purchase successful",
	}, nil
}

func (s *Server) TransferCoins(ctx context.Context, req *pb.TransferRequest) (*pb.TransferResponse, error) {
	senderIDVal := ctx.Value("userID")
	if senderIDVal == nil {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	senderID, ok := senderIDVal.(int)
	if !ok {
		return nil, status.Error(codes.Internal, "invalid userID in context")
	}

	if senderID == int(req.ToUser) {
		return nil, status.Error(codes.InvalidArgument, "cannot transfer to yourself")
	}

	if err := s.svc.TransferCoins(ctx, senderID, int(req.ToUser), int(req.Amount)); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "transfer failed: %v", err)
	}

	return &pb.TransferResponse{
		Success: true,
		Message: "transfer successful",
	}, nil
}

func (s *Server) GetInfo(ctx context.Context, req *pb.GetInfoRequest) (*pb.GetInfoResponse, error) {
	userIDVal := ctx.Value("userID")
	if userIDVal == nil {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}
	userID, ok := userIDVal.(int)
	if !ok {
		return nil, status.Error(codes.Internal, "invalid userID in context")
	}

	info, err := s.svc.GetInfo(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user info: %v", err)
	}

	var pbPurchases []*pb.Purchase
	for _, p := range info.Purchases {
		pbPurchases = append(pbPurchases, &pb.Purchase{
			Id:           int32(p.ID),
			MerchName:    p.MerchName,
			Price:        int32(p.Price),
			PurchaseDate: p.CreatedAt.Format(time.RFC3339),
		})
	}

	var pbTransactions []*pb.Transaction
	for _, t := range info.Transactions {
		pbTransactions = append(pbTransactions, &pb.Transaction{
			Id:         int32(t.ID),
			SenderId:   int32(t.SenderID),
			ReceiverId: int32(t.ReceiverID),
			Amount:     int32(t.Amount),
			CreatedAt:  t.CreatedAt.Format(time.RFC3339),
		})
	}

	userInfo := &pb.UserInfo{
		UserId:       int32(info.UserID),
		Username:     info.Username,
		Balance:      int32(info.Balance),
		Purchases:    pbPurchases,
		Transactions: pbTransactions,
	}

	return &pb.GetInfoResponse{Info: userInfo}, nil
}

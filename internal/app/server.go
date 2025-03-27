package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
	"merch-store-grpc/api/pb"
	"merch-store-grpc/internal/config"
	mygprc "merch-store-grpc/internal/controller/grpc"
	"merch-store-grpc/internal/controller/grpc/middleware"
	"merch-store-grpc/internal/service"
	"merch-store-grpc/internal/storage/cache"
	"merch-store-grpc/internal/storage/cache/redis"
	"merch-store-grpc/internal/storage/db"
	"merch-store-grpc/internal/storage/db/postgres"
	"merch-store-grpc/pkg/jwt"
	"merch-store-grpc/pkg/logger"
	"merch-store-grpc/pkg/password"
	"net"
	"net/http"
)

type Server struct {
	closer     *Closer
	pgPool     *pgxpool.Pool
	config     *config.Config
	grpcServer *grpc.Server
	logger     logger.Logger
	cache      cache.CacheRepository
}

func NewServer(cfg *config.Config, log logger.Logger) *Server {
	c := NewCloser()

	pgPool, err := cfg.Storage.ConnectionToPostgres(log)
	if err != nil {
		log.Fatalw("connect to postgres",
			"error", err)
	}
	c.Add(func(ctx context.Context) error {
		log.Infow("Closing PostgreSQL pool")
		pgPool.Close()
		return nil
	})

	clientRedis, err := cfg.Storage.ConnectionToRedis(log)
	if err != nil {
		log.Fatalw("connect to redis",
			"error", err)
	}

	c.Add(func(ctx context.Context) error {
		log.Infow("Closing Redis")
		clientRedis.Close()
		return nil
	})

	txManager := postgres.NewTxManager(pgPool, log)

	userRepo := postgres.NewUserRepository(txManager, log)
	purchaseRepo := postgres.NewPurchaseRepository(txManager, log)
	transactionRepo := postgres.NewTransactionRepository(txManager, log)

	repo := db.NewRepository(userRepo, purchaseRepo, transactionRepo)

	tokenService := jwt.NewTokenService(cfg.JWT.SecretKey, cfg.JWT.TokenExpiry)
	passwordHasher := password.NewBCryptHasher(0)

	cacheRepo := redis.NewRedisCacheRepository(clientRedis, log)

	svc := service.NewMerchStoreService(repo, cacheRepo, txManager, tokenService, passwordHasher, 1000, log)

	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.JWTUnaryInterceptor(tokenService)),
	)

	server := mygprc.NewServer(svc)

	pb.RegisterMerchServiceServer(grpcSrv, server)

	reflection.Register(grpcSrv)

	return &Server{
		closer:     c,
		config:     cfg,
		grpcServer: grpcSrv,
		pgPool:     pgPool,
		logger:     log,
		cache:      cacheRepo,
	}
}

// Run запускает gRPC-сервер и HTTP-прокси (grpc-gateway) в отдельных горутинах.
func (s *Server) Run(ctx context.Context) error {
	catalog := map[string]interface{}{
		"t-shirt":    80,
		"cup":        20,
		"book":       50,
		"pen":        10,
		"powerbank":  200,
		"hoody":      300,
		"umbrella":   200,
		"socks":      10,
		"wallet":     50,
		"pink-hoody": 500,
	}
	err := s.cache.LoadCatalog(ctx, catalog)
	if err != nil {
		s.logger.Fatalw("load catalog",
			"error", err)
	}

	// Запуск gRPC-сервера
	if err := s.runGRPC(ctx); err != nil {
		return err
	}

	// Запуск grpc-gateway (HTTP-прокси)
	if err := s.runGateway(ctx); err != nil {
		return err
	}

	return nil
}

func (s *Server) runGRPC(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.PublicServer.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.closer.Add(func(ctx context.Context) error {
		s.logger.Infow("Shutting down gRPC server")
		s.grpcServer.GracefulStop()
		return nil
	})

	go func() {
		s.logger.Infow("Starting gRPC server", "address", lis.Addr().String())
		if err := s.grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			s.logger.Fatalw("gRPC server error", "error", err)
		}
	}()
	return nil
}

func (s *Server) runGateway(ctx context.Context) error {
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
	)

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	endpoint := fmt.Sprintf("localhost:%d", s.config.PublicServer.Port)
	conn, err := grpc.DialContext(ctx, endpoint, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial gRPC endpoint: %w", err)
	}

	s.closer.Add(func(ctx context.Context) error {
		s.logger.Infow("Closing gRPC connection")
		return conn.Close()
	})

	if err := pb.RegisterMerchServiceHandler(ctx, mux, conn); err != nil {
		return fmt.Errorf("failed to register merch service handler: %w", err)
	}

	corsHandler := middleware.EnableCORS(mux)
	loggingHandler := middleware.LoggingMiddleware(corsHandler)

	fs := http.FileServer(http.Dir("./docs"))
	http.Handle("/swagger/", http.StripPrefix("/swagger/", fs))

	gwAddr := fmt.Sprintf(":%d", s.config.Gateway.Port)

	srv := &http.Server{
		Addr:    gwAddr,
		Handler: loggingHandler,
	}
	s.closer.Add(func(ctx context.Context) error {
		s.logger.Infow("Shutting down HTTP gateway")
		return srv.Shutdown(ctx)
	})

	go func() {
		s.logger.Infow("Starting HTTP gateway", "address", gwAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatalw("HTTP gateway error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.closer.Close(ctx)
}

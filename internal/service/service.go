package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5"
	"merch-store-grpc/internal/models"
	"merch-store-grpc/internal/storage/cache"
	"merch-store-grpc/internal/storage/db"
	"merch-store-grpc/internal/storage/db/postgres"
	"merch-store-grpc/pkg/jwt"
	"merch-store-grpc/pkg/logger"
	"merch-store-grpc/pkg/password"
	"time"
)

type MerchStoreService interface {
	Authenticate(ctx context.Context, username, password string) (string, error)
	PurchaseMerch(ctx context.Context, userID int, merchName string) error
	TransferCoins(ctx context.Context, fromUser, toUser, amount int) error
	GetInfo(ctx context.Context, userID int) (*models.UserInfo, error)
}

type merchStoreServiceImp struct {
	repo           db.Repository
	cacheRepo      cache.CacheRepository
	txManager      db.TxManager
	tokenService   jwt.TokenService
	passwordHasher password.PasswordHasher
	initialBalance int
	log            logger.Logger
}

func NewMerchStoreService(
	repo db.Repository,
	cacheRepo cache.CacheRepository,
	txManager db.TxManager,
	tokenService jwt.TokenService,
	passwordHasher password.PasswordHasher,
	initialBalance int,
	log logger.Logger,
) MerchStoreService {
	return &merchStoreServiceImp{
		repo:           repo,
		cacheRepo:      cacheRepo,
		txManager:      txManager,
		tokenService:   tokenService,
		passwordHasher: passwordHasher,
		initialBalance: initialBalance,
		log:            log,
	}
}

func (s *merchStoreServiceImp) Authenticate(ctx context.Context, username, password string) (string, error) {
	user, err := s.getOrCreateUser(ctx, username, password)
	if err != nil {
		return "", err
	}

	token, err := s.tokenService.GenerateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func (s *merchStoreServiceImp) getOrCreateUser(ctx context.Context, username, password string) (*models.User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if user == nil {
		return s.createNewUser(ctx, username, password)
	}

	if !s.passwordHasher.Check(user.PasswordHash, password) {
		return nil, errors.New("invalid credentials")
	}

	_, err = s.cacheRepo.GetBalance(ctx, user.ID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			if setErr := s.cacheRepo.SetBalance(ctx, user.ID, user.Balance); setErr != nil {
				s.log.Errorw("set balance in cache", "userID", user.ID, "error", setErr)
			}
		} else {
			s.log.Errorw("get balance from cache", "userID", user.ID, "error", err)
		}
	}

	return user, nil
}

func (s *merchStoreServiceImp) createNewUser(ctx context.Context, username, password string) (*models.User, error) {
	hashedPwd, err := s.passwordHasher.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	newUser := &models.User{
		Username:     username,
		PasswordHash: hashedPwd,
		Balance:      s.initialBalance,
		CreatedAt:    time.Now(),
	}

	userID, err := s.repo.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}

	if err := s.cacheRepo.SetBalance(ctx, userID, s.initialBalance); err != nil {
		return nil, fmt.Errorf("set balance in cache: %w", err)
	}

	return &models.User{
		ID:           userID,
		Username:     username,
		PasswordHash: hashedPwd,
		Balance:      s.initialBalance,
	}, nil
}

func (s *merchStoreServiceImp) PurchaseMerch(ctx context.Context, userID int, merchName string) error {
	price, err := s.cacheRepo.GetPrice(ctx, merchName)
	if err != nil {
		return fmt.Errorf("get merch price: %w", err)
	}
	currentBalance, err := s.cacheRepo.GetBalance(ctx, userID)
	if err != nil {
		return fmt.Errorf("get balance: %w", err)
	}
	if currentBalance < price {
		return errors.New("insufficient funds")
	}

	err = s.txManager.WithTx(ctx, pgx.Serializable, pgx.ReadWrite, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		if user.Balance < price {
			return errors.New("insufficient funds (DB)")
		}
		newBalance := user.Balance - price
		if err := s.repo.UpdateBalance(txCtx, userID, newBalance); err != nil {
			return err
		}
		purchase := &models.Purchase{
			UserID:    userID,
			MerchName: merchName,
			Price:     price,
			CreatedAt: time.Now(),
		}

		if _, err := s.repo.CreatePurchase(txCtx, purchase); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.cacheRepo.DeductBalance(ctx, userID, price); err != nil {
		return fmt.Errorf("purchase succeeded but failed to update cache: %w", err)
	}
	return nil
}

func (s *merchStoreServiceImp) TransferCoins(ctx context.Context, fromUser, toUser, amount int) error {
	if amount <= 0 {
		return fmt.Errorf("transfer amount: amount must be positive")
	}

	if fromUser == toUser {
		return fmt.Errorf("cannot transfer to yourself")
	}
	senderBalance, err := s.cacheRepo.GetBalance(ctx, fromUser)
	if err != nil {
		return fmt.Errorf("get sender balance: %w", err)
	}
	if senderBalance < amount {
		return errors.New("insufficient funds for transfer")
	}

	err = s.txManager.WithTx(ctx, pgx.Serializable, pgx.ReadWrite, func(txCtx context.Context) error {
		sender, err := s.repo.GetUserByID(txCtx, fromUser)
		if err != nil {
			return err
		}
		if sender.Balance < amount {
			return errors.New("insufficient funds in DB for sender")
		}
		receiver, err := s.repo.GetUserByID(txCtx, toUser)
		if err != nil {
			return err
		}
		newSenderBal := sender.Balance - amount
		newReceiverBal := receiver.Balance + amount
		if err := s.repo.UpdateBalance(txCtx, fromUser, newSenderBal); err != nil {
			return err
		}
		if err := s.repo.UpdateBalance(txCtx, toUser, newReceiverBal); err != nil {
			return err
		}
		txRecord := &models.Transaction{
			SenderID:   fromUser,
			ReceiverID: toUser,
			Amount:     amount,
			CreatedAt:  time.Now(),
		}
		if _, err := s.repo.CreateTransaction(txCtx, txRecord); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.cacheRepo.TransferCoins(ctx, fromUser, toUser, amount); err != nil {
		return fmt.Errorf("transfer succeeded but failed to update cache: %w", err)
	}
	return nil
}

func (s *merchStoreServiceImp) GetInfo(ctx context.Context, userID int) (*models.UserInfo, error) {
	var result *models.UserInfo

	err := s.txManager.WithTx(ctx, postgres.IsolationLevelReadCommitted, postgres.AccessModeReadOnly, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}

		purchases, err := s.repo.GetPurchaseByUserID(txCtx, userID)
		if err != nil {
			return err
		}

		transactions, err := s.repo.GetTransactionByUserID(txCtx, userID)
		if err != nil {
			return err
		}

		result = &models.UserInfo{
			UserID:       user.ID,
			Username:     user.Username,
			Balance:      user.Balance,
			Purchases:    purchases,
			Transactions: transactions,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

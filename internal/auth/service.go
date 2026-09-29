package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
	"github.com/aliefbuscode/task-management-api/internal/middleware"
)

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

type Service struct {
	repo             Repository
	jwtSecret        string
	jwtExpiryMinutes int
}

func NewService(repo Repository, jwtSecret string, jwtExpiryMinutes int) *Service {
	return &Service{repo: repo, jwtSecret: jwtSecret, jwtExpiryMinutes: jwtExpiryMinutes}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, *apperror.AppError) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}

	teamID := req.TeamID
	if teamID == 0 {
		teamID = 1
	}

	user := &User{
		TeamID:       teamID,
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				return nil, apperror.ErrConflict("email already registered")
			case pgForeignKeyViolation:
				return nil, apperror.ErrValidation("team_id does not exist")
			}
		}
		return nil, apperror.ErrInternal(err)
	}

	return s.issueToken(user)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, *apperror.AppError) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if errors.Is(err, ErrUserNotFound) {
		return nil, apperror.New(401, "INVALID_CREDENTIALS", "email or password is incorrect")
	}
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperror.New(401, "INVALID_CREDENTIALS", "email or password is incorrect")
	}

	return s.issueToken(user)
}

func (s *Service) issueToken(user *User) (*AuthResponse, *apperror.AppError) {
	expiry := time.Duration(s.jwtExpiryMinutes) * time.Minute
	claims := middleware.Claims{
		UserID: user.ID,
		TeamID: user.TeamID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, apperror.ErrInternal(err)
	}

	return &AuthResponse{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresIn:   int(expiry.Seconds()),
		User:        *user,
	}, nil
}

package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aliefbuscode/task-management-api/internal/apperror"
)

type Claims struct {
	UserID int64 `json:"user_id"`
	TeamID int64 `json:"team_id"`
	jwt.RegisteredClaims
}

// JWTAuth validates the Authorization: Bearer <token> header and, on
// success, stores user_id/team_id in c.Locals for downstream handlers.
func JWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return apperror.ErrUnauthorized("missing or malformed authorization header")
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return apperror.ErrUnauthorized("invalid or expired token")
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("team_id", claims.TeamID)
		AddField(c, "user_id", claims.UserID)
		AddField(c, "team_id", claims.TeamID)
		return c.Next()
	}
}

func UserID(c *fiber.Ctx) int64 {
	if v, ok := c.Locals("user_id").(int64); ok {
		return v
	}
	return 0
}

func TeamID(c *fiber.Ctx) int64 {
	if v, ok := c.Locals("team_id").(int64); ok {
		return v
	}
	return 0
}

package auth

import "time"

type User struct {
	ID           int64     `db:"id" json:"id"`
	TeamID       int64     `db:"team_id" json:"team_id"`
	Name         string    `db:"name" json:"name"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

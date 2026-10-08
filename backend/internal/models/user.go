package models

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
}

type Session struct {
	User      User
	ExpiresAt time.Time
}

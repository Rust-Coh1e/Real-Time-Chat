package internal

import "github.com/google/uuid"

type Oper string

// const (
// 	OperDeposit  Oper = "DEPOSIT"
// 	OperWithDraw Oper = "WITHDRAW"
// )

type CreateUserOperation struct {
	// id          UUID PRIMARY KEY,
	// username    TEXT NOT NULL UNIQUE,
	// password_hash      TEXT NOT NULL,
	// created_at  TIMESTAMP DEFAULT NOW()

	UserID   uuid.UUID
	Username string
	Password string
}

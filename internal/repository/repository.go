package repository

import (
	"waap-backend/pkg/db"
)

// Repositories aggregates domain repositories.
type Repositories struct {
	Users UserRepository
}

// NewRepositories constructs all repositories wrapping db.Querier.
func NewRepositories(queries db.Querier) *Repositories {
	return &Repositories{
		Users: NewUserRepository(queries),
	}
}

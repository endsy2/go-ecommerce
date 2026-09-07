// Package repository is the only place GORM is imported. It takes and returns
// models, and it translates database errors into the domain vocabulary so that
// nothing above it has to know what an ORM is.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/model"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByEmail looks a user up case-insensitively.
//
// The predicate is written as lower(email) = lower(?) to match the expression
// index the migration creates (UNIQUE on lower(email)). Written as
// `email = ?` the query would still be correct, but Postgres could not use that
// index and would fall back to a sequential scan on every login attempt.
//
// A missing row comes back as domain.ErrNotFound, not gorm.ErrRecordNotFound:
// the service layer must not have to import GORM to interpret a result.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User

	err := r.db.WithContext(ctx).
		Where("lower(email) = lower(?)", email).
		First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No email in the message: this error travels toward a login response,
		// and the whole point of that endpoint is not to confirm which
		// addresses are registered.
		return nil, domain.NotFoundf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("finding user by email: %w", err)
	}

	return &user, nil
}

// FindByID looks a user up by primary key.
//
// The lookup is by uuid rather than email, so unlike FindByEmail there is no
// enumeration concern here: the caller already holds a signed token naming this
// id. A miss means the row was deleted after the token was issued.
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User

	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.NotFoundf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("finding user by id %s: %w", id, err)
	}

	return &user, nil
}

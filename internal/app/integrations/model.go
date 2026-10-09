package integrations

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("integration not found")

type Integration struct {
	ID          int64
	Name        string
	Provider    string
	Credentials Credentials
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Credentials struct {
	Type  string
	Token string
	Email string
	Key   string
}

type CreateInput struct {
	Name        string
	Provider    string
	Credentials Credentials
}

type UpdateInput struct {
	Name        string
	Credentials Credentials
}

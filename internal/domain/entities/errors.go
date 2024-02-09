package entities

import "github.com/pkg/errors"

var (
	ErrInvalidEmail = errors.New("invalid email")
	ErrInvalidIP    = errors.New("invalid IP address")
)

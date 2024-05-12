package entities

import "errors"

var (
	ErrInvalidEmail = errors.New("invalid email")
	ErrInvalidIP    = errors.New("invalid IP address")
)

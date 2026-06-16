package entities

import "errors"

var (
	ErrInvalidEmail = errors.New("invalid email")
	ErrInvalidIP    = errors.New("invalid IP address")
	// ErrLabelNotValidInPhase is returned when a label is not valid in a phase
	ErrLabelNotValidInPhase = errors.New("label is not valid in this phase")
)

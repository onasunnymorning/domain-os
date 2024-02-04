package commands

import "time"

// CreateNNDNCommandResult is the result of the CreateNNDNCommand
type CreateNNDNCommandResult struct {
	Result NNDNResult
}

// CreateNNDNCommand is the command to create a NNDN
type CreateNNDNCommand struct {
	Name string
}

// NNDNResult is the result converting an entity NNDN to a command NNDNResult
type NNDNResult struct {
	Name      string
	Type      string
	UName     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

package commands

import (
	"reflect"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestChunkCreateRegistrarCommands(t *testing.T) {
	tests := []struct {
		name      string
		cmds      []CreateRegistrarCommand
		chunkSize int
		expected  [][]CreateRegistrarCommand
	}{
		{
			name: "chunk size 1",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 1,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}},
				{{ClID: "2"}},
				{{ClID: "3"}},
			},
		},
		{
			name: "chunk size 2",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 2,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}, {ClID: "2"}},
				{{ClID: "3"}},
			},
		},
		{
			name: "chunk size greater than length",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 5,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}, {ClID: "2"}, {ClID: "3"}},
			},
		},
		{
			name: "chunk size zero",
			cmds: []CreateRegistrarCommand{
				{ClID: "1"}, {ClID: "2"}, {ClID: "3"},
			},
			chunkSize: 0,
			expected: [][]CreateRegistrarCommand{
				{{ClID: "1"}},
				{{ClID: "2"}},
				{{ClID: "3"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := ChunkCreateRegistrarCommands(tt.cmds, tt.chunkSize)
			var result [][]CreateRegistrarCommand
			for chunk := range ch {
				result = append(result, chunk)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFromIANARegistrar(t *testing.T) {
	tests := []struct {
		name      string
		registrar entities.IANARegistrar
		expected  CreateRegistrarCommand
	}{
		{
			name: "basic test",
			registrar: entities.IANARegistrar{
				Name: "Test Registrar",
			},
			expected: CreateRegistrarCommand{
				Name: "Test Registrar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromIANARegistrar(tt.registrar)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

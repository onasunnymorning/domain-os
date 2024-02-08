package repositories

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type TLDRepository interface {
	Create(tld *entities.TLD) error
	GetByName(name string) (*entities.TLD, error)
	List(pageSize int, pageCursor string) ([]*entities.TLD, error)
	// Update(tld *entities.TLD) error
	DeleteByName(name string) error
}

package repos

import "github.com/onasunnymorning/registry-os/internal/domain/entities"

type TLDRepo interface {
	Create(tld *entities.TLD) error
	GetByName(name string) (*entities.TLD, error)
	List(pageSize int, pageCursor string) ([]*entities.TLD, error)
	// Update(tld *entities.TLD) error
	DeleteByName(name string) error
}

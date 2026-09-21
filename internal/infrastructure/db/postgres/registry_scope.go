package postgres

import (
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
)

// scopeToTLDs constrains q, a query on a table with a tld_name column, to the
// TLDs scope may act on. The platform acts on every TLD; an operator only on
// the TLDs whose ry_id is its own (ADR-0006). A row outside the scope is simply
// not matched, so to the caller it is indistinguishable from a row that does
// not exist — which is what isolation means here (#415).
//
// The subquery is for single-row statements. It costs one lookup in a table of
// a few hundred TLDs. A bulk statement over millions of rows should resolve the
// TLD once with GetByNameForOperator and filter on a plain tld_name = ?
// instead, rather than carry a semi-join through the whole delete.
func scopeToTLDs(q *gorm.DB, scope entities.RegistryScope) (*gorm.DB, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if scope.IsPlatform() {
		return q, nil
	}
	return q.Where("tld_name IN (SELECT name FROM tlds WHERE ry_id = ?)", scope.Operator().String()), nil
}

// deletedOrNotFound turns a delete that matched nothing into notFound. Without
// this a scoped delete of someone else's row would report success while the row
// is still there.
func deletedOrNotFound(res *gorm.DB, notFound error) error {
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notFound
	}
	return nil
}

package postgres

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// errNotWritten is a conditional write that matched no row. Each repository
// turns it into the object's own error: gone, or changed since it was read.
var errNotWritten = errors.New("no row matched the conditional write")

// updateIfAt narrows q to the row still at version read, its updated_at as
// the caller read it. A row never updated may have none.
func updateIfAt(q *gorm.DB, read time.Time) *gorm.DB {
	if read.IsZero() {
		return q.Where("updated_at IS NULL OR updated_at = ?", read)
	}
	return q.Where("updated_at = ?", read)
}

// written is the error of a conditional write: errNotWritten when it matched
// no row.
func written(res *gorm.DB) error {
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errNotWritten
	}
	return nil
}

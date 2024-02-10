package postgres

// HostAddress is the GORM model for the host_address table
type HostAddress struct {
	ID       int64 `gorm:"primaryKey"`
	Version  int
	IP       string
	HostRoID int64
}

// TableName returns the table name for the HostAddress model
func (HostAddress) TableName() string {
	return "host_addresses"
}

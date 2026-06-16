package repositories

// IDGenerator is an interface for generating unique IDs
type IDGenerator interface {
	GenerateID() int64
	ListNode() int64
}

package queries

// ListItemsFilter is the interface that contains the filter for the list items query
type ListItemsFilter interface {
	ToQueryParams() string
}

// ListItemsQuery is the struct that contains the query for the list items query
type ListItemsQuery struct {
	PageSize   int
	PageCursor string
	Filter     ListItemsFilter
}

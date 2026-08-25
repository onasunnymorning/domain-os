package queries

type ActiveDomainsWithHostsQuery struct {
	TldName    string
	PageSize   int
	PageCursor string
}

package response

import "time"

type CanAutoRenewResponse struct {
	DomainName   string
	CanAutoRenew bool
	Timestamp    time.Time
}

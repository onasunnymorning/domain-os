package queries

// ListPremiumLabelsFilter is the struct that contains the filter for the list premium labels query
type ListPremiumLabelsFilter struct {
	LabelLike                string
	PremiumListNameEquals    string
	RegistrationAmountEquals string
	RenewalAmountEquals      string
	TransferAmountEquals     string
	RestoreAmountEquals      string
	CurrencyEquals           string
	ClassEquals              string
}

// ToQueryParams converts the Filter to a query string that can be appended to the URL
func (f ListPremiumLabelsFilter) ToQueryParams() string {
	queryString := ""
	if f.LabelLike != "" {
		queryString += "&label_like=" + f.LabelLike
	}
	if f.PremiumListNameEquals != "" {
		queryString += "&premium_list_name_equals=" + f.PremiumListNameEquals
	}
	if f.RegistrationAmountEquals != "" {
		queryString += "&registration_amount_equals=" + f.RegistrationAmountEquals
	}
	if f.RenewalAmountEquals != "" {
		queryString += "&renewal_amount_equals=" + f.RenewalAmountEquals
	}
	if f.TransferAmountEquals != "" {
		queryString += "&transfer_amount_equals=" + f.TransferAmountEquals
	}
	if f.RestoreAmountEquals != "" {
		queryString += "&restore_amount_equals=" + f.RestoreAmountEquals
	}
	if f.CurrencyEquals != "" {
		queryString += "&currency_equals=" + f.CurrencyEquals
	}
	if f.ClassEquals != "" {
		queryString += "&class_equals=" + f.ClassEquals
	}

	return queryString
}

package queries

import "testing"

func TestPremiumLabelToQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		filter   ListPremiumLabelsFilter
		expected string
	}{
		{
			name:     "all empty fields",
			filter:   ListPremiumLabelsFilter{},
			expected: "",
		},
		{
			name: "single field set",
			filter: ListPremiumLabelsFilter{
				LabelLike: "example",
			},
			expected: "&label_like=example",
		},
		{
			name: "multiple fields set",
			filter: ListPremiumLabelsFilter{
				LabelLike:                "example",
				PremiumListNameEquals:    "premium",
				RegistrationAmountEquals: "100",
				RenewalAmountEquals:      "200",
				TransferAmountEquals:     "300",
				RestoreAmountEquals:      "400",
				CurrencyEquals:           "USD",
				ClassEquals:              "A",
			},
			expected: "&label_like=example" +
				"&premium_list_name_equals=premium" +
				"&registration_amount_equals=100" +
				"&renewal_amount_equals=200" +
				"&transfer_amount_equals=300" +
				"&restore_amount_equals=400" +
				"&currency_equals=USD" +
				"&class_equals=A",
		},
		{
			name: "fields with empty and non-empty values",
			filter: ListPremiumLabelsFilter{
				LabelLike:             "test",
				PremiumListNameEquals: "",
				CurrencyEquals:        "EUR",
			},
			expected: "&label_like=test" + "&currency_equals=EUR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.filter.ToQueryParams()
			if result != tc.expected {
				t.Errorf("Expected %q but got %q", tc.expected, result)
			}
		})
	}
}

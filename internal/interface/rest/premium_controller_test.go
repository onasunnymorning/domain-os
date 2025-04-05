package rest

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
)

func TestGetPremiumLabelFilterFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryString    string
		expectedFilter queries.ListPremiumLabelsFilter
	}{
		{
			name:        "all filters provided",
			queryString: "label_like=test&premium_list_name_equals=PremiumTest&currency_equals=usd&class_equals=gold&registration_amount_equals=100&renewal_amount_equals=50&transfer_amount_equals=25&restore_amount_equals=10",
			expectedFilter: queries.ListPremiumLabelsFilter{
				LabelLike:                "test",
				PremiumListNameEquals:    "PremiumTest",
				CurrencyEquals:           "USD",
				ClassEquals:              "gold",
				RegistrationAmountEquals: "100",
				RenewalAmountEquals:      "50",
				TransferAmountEquals:     "25",
				RestoreAmountEquals:      "10",
			},
		},
		{
			name:        "empty filters",
			queryString: "",
			expectedFilter: queries.ListPremiumLabelsFilter{
				LabelLike:                "",
				PremiumListNameEquals:    "",
				CurrencyEquals:           "",
				ClassEquals:              "",
				RegistrationAmountEquals: "",
				RenewalAmountEquals:      "",
				TransferAmountEquals:     "",
				RestoreAmountEquals:      "",
			},
		},
		{
			name:        "only currency filter provided",
			queryString: "currency_equals=eur",
			expectedFilter: queries.ListPremiumLabelsFilter{
				LabelLike:                "",
				PremiumListNameEquals:    "",
				CurrencyEquals:           "EUR",
				ClassEquals:              "",
				RegistrationAmountEquals: "",
				RenewalAmountEquals:      "",
				TransferAmountEquals:     "",
				RestoreAmountEquals:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tt.queryString, nil)
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = req

			filter, err := getPremiumLabelFilterFromContext(ctx)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if filter != tt.expectedFilter {
				t.Errorf("expected filter %+v, got %+v", tt.expectedFilter, filter)
			}
		})
	}
}

func TestGetPremiumListFilterFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryString    string
		expectedFilter queries.ListPremiumListsFilter
	}{
		{
			name:        "all filters provided",
			queryString: "name_like=test&ryid_equals=123&created_before=2023-10-10&created_after=2023-01-01",
			expectedFilter: queries.ListPremiumListsFilter{
				NameLike:      "test",
				RyIDEquals:    "123",
				CreatedBefore: "2023-10-10",
				CreatedAfter:  "2023-01-01",
			},
		},
		{
			name:           "empty filters",
			queryString:    "",
			expectedFilter: queries.ListPremiumListsFilter{},
		},
		{
			name:        "only one filter provided",
			queryString: "name_like=foo",
			expectedFilter: queries.ListPremiumListsFilter{
				NameLike: "foo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?"+tt.queryString, nil)
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = req

			filter, err := getPremiumListFilterFromContext(ctx)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if filter != tt.expectedFilter {
				t.Errorf("expected filter %+v, got %+v", tt.expectedFilter, filter)
			}
		})
	}
}

package fixer

// {
//     "success": true,
//     "timestamp": 1519296206,
//     "base": "EUR",
//     "date": "2024-05-14",
//     "rates": {
//         "AUD": 1.566015,
//         "CAD": 1.560132,
//         "CHF": 1.154727,
//         "CNY": 7.827874,
//         "GBP": 0.882047,
//         "JPY": 132.360679,
//         "USD": 1.23396,
//     [...]
//     }
// }

// LatestRatesResponse represents the response from the Fixer API's 'Latest Rates' endpoint
// Ref. https://fixer.io/documentation
type LatestRatesResponse struct {
	Success   bool               `json:"success"`
	Timestamp int64              `json:"timestamp"`
	Base      string             `json:"base"`
	Date      string             `json:"date"`
	Rates     map[string]float64 `json:"rates"`
}

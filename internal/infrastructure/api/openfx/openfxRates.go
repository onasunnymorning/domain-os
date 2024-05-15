package openfx

// {
//     "disclaimer": "Usage subject to terms: https://openexchangerates.org/terms",
//     "license": "https://openexchangerates.org/license",
//     "timestamp": 1715774400,
//     "base": "USD",
//     "rates": {
//         "EUR": 0.923402,
//         "PEN": 3.719933
//     }
// }

// LatestRatesResponse represents the response from the Fixer API's 'Latest Rates' endpoint
// Ref. https://fixer.io/documentation
type LatestRatesResponse struct {
	Timestamp int64              `json:"timestamp"`
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
}

package response

import "time"

// IsAccreditedResponse is the response for the IsAccredited endpoint including the registrar ClID, TLD name, accreditation status, and timestamp
type IsAccreditedResponse struct {
	RegistrarClID string    `json:"registrarClID"`
	TLDName       string    `json:"tldName"`
	IsAccredited  bool      `json:"isAccredited"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewIsAccreditedResponse returns a new IsAccreditedResponse
func NewIsAccreditedResponse(registrarClID, tldName string) *IsAccreditedResponse {
	return &IsAccreditedResponse{
		RegistrarClID: registrarClID,
		TLDName:       tldName,
		Timestamp:     time.Now().UTC(),
	}
}

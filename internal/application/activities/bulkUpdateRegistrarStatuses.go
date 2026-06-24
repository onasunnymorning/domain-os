package activities

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
)

// BulkUpdateResult captures the outcome of a bulk status update operation.
type BulkUpdateResult struct {
	Updated    int
	Failed     int
	UpdatedIDs []string
	Errors     []BulkUpdateError
}

// BulkUpdateError records a single status update failure.
type BulkUpdateError struct {
	ClID      string
	Operation string
	Error     string
}

// BulkUpdateRegistrarStatuses applies a batch of registrar status updates sequentially.
// For each command, it updates platform status and/or IANA status via the REST API,
// only making HTTP calls for fields that actually changed (NewStatus / NewIANAStatus non-empty).
// Failures are collected rather than aborting, so the workflow gets a complete picture.
func BulkUpdateRegistrarStatuses(correlationID string, updates []commands.UpdateRegistrarStatusCommand) (BulkUpdateResult, error) {
	result := BulkUpdateResult{
		UpdatedIDs: []string{},
		Errors:     []BulkUpdateError{},
	}

	client := http.Client{}

	for _, upd := range updates {
		failed := false

		// Update platform status if changed
		if upd.NewStatus != "" {
			if err := updateStatus(client, correlationID, upd.ClID, upd.NewStatus); err != nil {
				result.Errors = append(result.Errors, BulkUpdateError{
					ClID:      upd.ClID,
					Operation: "update-status",
					Error:     err.Error(),
				})
				failed = true
			}
		}

		// Update IANA status if changed
		if upd.NewIANAStatus != "" {
			if err := updateIANAStatus(client, correlationID, upd.ClID, upd.NewIANAStatus); err != nil {
				result.Errors = append(result.Errors, BulkUpdateError{
					ClID:      upd.ClID,
					Operation: "update-iana-status",
					Error:     err.Error(),
				})
				failed = true
			}
		}

		if failed {
			result.Failed++
		} else {
			result.Updated++
			result.UpdatedIDs = append(result.UpdatedIDs, upd.ClID)
		}
	}

	return result, nil
}

// updateStatus sends a PUT request to update a registrar's platform status.
func updateStatus(client http.Client, correlationID, clid, status string) error {
	endpoint := fmt.Sprintf("%s/registrars/%s/status/%s", BASEURL, clid, strings.ToLower(status))

	qParams := map[string]string{"correlationID": correlationID}
	url, err := getURLAndSetQueryParams(endpoint, qParams)
	if err != nil {
		return fmt.Errorf("failed to build URL for status update (clid=%s): %w", clid, err)
	}

	req, err := http.NewRequest("PUT", url.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request for status update (clid=%s): %w", clid, err)
	}
	req.Header.Add("Authorization", GetBearerToken())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set registrar %s status to %s: %w", clid, status, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to set registrar %s status to %s: HTTP %s", clid, status, resp.Status)
	}

	return nil
}

// updateIANAStatus sends a PUT request to update a registrar's IANA status.
func updateIANAStatus(client http.Client, correlationID, clid, ianaStatus string) error {
	endpoint := fmt.Sprintf("%s/registrars/%s/iana_status/%s", BASEURL, clid, ianaStatus)

	qParams := map[string]string{"correlationID": correlationID}
	url, err := getURLAndSetQueryParams(endpoint, qParams)
	if err != nil {
		return fmt.Errorf("failed to build URL for IANA status update (clid=%s): %w", clid, err)
	}

	req, err := http.NewRequest("PUT", url.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request for IANA status update (clid=%s): %w", clid, err)
	}
	req.Header.Add("Authorization", GetBearerToken())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set registrar %s IANA status to %s: %w", clid, ianaStatus, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Treat missing registrar as no-op
		return nil
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to set registrar %s IANA status to %s: HTTP %s", clid, ianaStatus, resp.Status)
	}

	return nil
}

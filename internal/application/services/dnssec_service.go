package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

type DnssecService struct{}

func NewDnssecService() *DnssecService {
	return &DnssecService{}
}

func (s *DnssecService) Visualize(ctx context.Context, domain string) (map[string]interface{}, error) {
	if !domainRegex.MatchString(domain) {
		return nil, fmt.Errorf("invalid domain format")
	}

	// Create temp directory for execution to isolate files
	tempDir, err := os.MkdirTemp("", "dnssec-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // clean up afterwards

	probeOutput := filepath.Join(tempDir, "probe.json")
	grokOutput := filepath.Join(tempDir, "grok.json")

	// 1. Run dnsviz probe with authoritative analysis (-A) to bypass Docker's recursive DNS
	probeCmd := exec.CommandContext(ctx, "dnsviz", "probe", "-A", "-a", ".", "-o", probeOutput, domain)
	if output, err := probeCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("dnsviz probe failed: %v, output: %s", err, string(output))
	}

	// 2. Run dnsviz grok
	grokCmd := exec.CommandContext(ctx, "dnsviz", "grok", "-r", probeOutput, "-o", grokOutput)
	if output, err := grokCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("dnsviz grok failed: %v, output: %s", err, string(output))
	}

	// 3. Read grok output
	content, err := os.ReadFile(grokOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to read grok output: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal grok output: %v", err)
	}

	return result, nil
}

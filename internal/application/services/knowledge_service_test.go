package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestNewKnowledgeService_LoadsFromIndex — integration test with a temp
// directory containing a mini corpus manifest and markdown files.
// ---------------------------------------------------------------------------

func TestNewKnowledgeService_LoadsFromIndex(t *testing.T) {
	dir := t.TempDir()

	// Create a mini docs/index.yaml
	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	manifest := `sources:
  system-docs:
    - docs/test_guide.md
  root-docs:
    - README.md
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	// Create test markdown files
	guide := `# Test Guide

## Getting Started

This is the getting started section with enough words to exceed the minimum
chunk size threshold of ten words for proper indexing.

## Advanced Usage

Advanced usage covers topics like performance tuning, scaling strategies,
deployment pipelines, and monitoring observability dashboards.
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "test_guide.md"), []byte(guide), 0o644))

	readme := `# Project README

## Overview

This project is a domain registry management system that handles domain
lifecycle operations including registration renewal expiry and purge.
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	assert.Equal(t, 2, svc.DocCount(), "should have loaded 2 documents")
	assert.GreaterOrEqual(t, svc.ChunkCount(), 3, "should have at least 3 chunks (2 from guide + 1 from readme)")
}

// ---------------------------------------------------------------------------
// TestNewKnowledgeService_GlobPattern — verify glob expansion works.
// ---------------------------------------------------------------------------

func TestNewKnowledgeService_GlobPattern(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	// Create workflow docs dir and files
	wfDir := filepath.Join(dir, "internal", "application", "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	wfDoc := `# Test Workflow

## Overview

This workflow handles the import of escrow data files from external registries
into the staging database for quality assurance validation.

## Step Breakdown

Step one validates the source file. Step two parses and extracts the assets
into individual CSV files for processing by the collation engine.
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "testWorkflow.doc.md"), []byte(wfDoc), 0o644))

	manifest := `sources:
  workflow-docs:
    glob: "internal/application/workflows/*.doc.md"
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	assert.Equal(t, 1, svc.DocCount())
	assert.GreaterOrEqual(t, svc.ChunkCount(), 2)
}

// ---------------------------------------------------------------------------
// TestKnowledgeService_Search_FindsRelevantChunk
// ---------------------------------------------------------------------------

func TestKnowledgeService_Search_FindsRelevantChunk(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	manifest := `sources:
  root-docs:
    - docs/arch.md
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	doc := `# Architecture

## Database Layer

The database layer uses PostgreSQL with GORM as the ORM framework.
Connection pooling is handled by pgxpool with configurable maximum connections.

## Temporal Integration

Temporal is used for workflow orchestration. Workers poll task queues for
workflow and activity tasks. Each queue is sized for its workload profile.
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "arch.md"), []byte(doc), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	results, err := svc.Search("PostgreSQL database GORM", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results, "should find at least one result")

	assert.Equal(t, "docs/arch.md", results[0].DocPath)
	assert.Equal(t, "Database Layer", results[0].Section)
}

// ---------------------------------------------------------------------------
// TestKnowledgeService_Search_NoResults
// ---------------------------------------------------------------------------

func TestKnowledgeService_Search_NoResults(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	manifest := `sources:
  root-docs:
    - docs/small.md
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	doc := `# Small Doc

## Content

This document contains information about domain registration lifecycle
management including renewal expiry redemption and purge operations.
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "small.md"), []byte(doc), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	results, err := svc.Search("xylophone quantum entanglement", 5)
	require.NoError(t, err)
	assert.Empty(t, results, "should return empty results for non-matching terms")
}

// ---------------------------------------------------------------------------
// TestKnowledgeService_Search_BM25Ranking — the chunk mentioning a term
// more frequently should rank higher.
// ---------------------------------------------------------------------------

func TestKnowledgeService_Search_BM25Ranking(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	manifest := `sources:
  root-docs:
    - docs/ranking.md
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	doc := `# Ranking Test

## Section A

Escrow escrow escrow escrow escrow. The escrow import process handles
large data files. Escrow deposits are validated before ingestion.

## Section B

The deployment pipeline uses Docker containers and Kubernetes orchestration
for reliable production releases. One mention of escrow here.
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "ranking.md"), []byte(doc), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	results, err := svc.Search("escrow", 5)
	require.NoError(t, err)
	require.Len(t, results, 2, "should match both sections")

	assert.Equal(t, "Section A", results[0].Section, "Section A should rank higher (more mentions)")
	assert.Greater(t, results[0].Score, results[1].Score, "first result should have higher BM25 score")
}

// ---------------------------------------------------------------------------
// TestChunkMarkdown — verify heading-based chunking and section trails.
// ---------------------------------------------------------------------------

func TestChunkMarkdown(t *testing.T) {
	content := `# Top Level Title

Some preamble text that should be captured as part of the first section
with enough words to pass the minimum chunk size threshold.

## First Section

Content for the first section with sufficient words to be indexed
by the knowledge service chunking logic properly.

### Subsection A

Detailed subsection content about configuration management strategies
for production deployment environments with many words.

## Second Section

Content for the second section with enough detail about workflow
orchestration patterns and temporal queue architecture.
`

	chunks := chunkMarkdown(content, "docs/test.md")

	require.GreaterOrEqual(t, len(chunks), 3, "should produce at least 3 chunks")

	// Verify we have the expected sections
	sectionNames := make([]string, len(chunks))
	for i, c := range chunks {
		sectionNames[i] = c.Section
	}

	assert.Contains(t, sectionNames, "First Section")
	assert.Contains(t, sectionNames, "First Section > Subsection A")
	assert.Contains(t, sectionNames, "Second Section")

	// Verify all chunks have the correct doc path
	for _, c := range chunks {
		assert.Equal(t, "docs/test.md", c.DocPath)
	}
}

// ---------------------------------------------------------------------------
// TestChunkMarkdown_NoHeadings — document without headings becomes one chunk.
// ---------------------------------------------------------------------------

func TestChunkMarkdown_NoHeadings(t *testing.T) {
	content := `This is a plain text document without any markdown headings.
It contains enough words to pass the minimum chunk threshold for indexing
by the knowledge service. The content discusses domain lifecycle operations.`

	chunks := chunkMarkdown(content, "notes.txt")

	require.Len(t, chunks, 1)
	assert.Equal(t, "notes.txt", chunks[0].Section)
	assert.Contains(t, chunks[0].Content, "plain text document")
}

// ---------------------------------------------------------------------------
// TestTokenize — lowercasing, stop word removal, special char splitting.
// ---------------------------------------------------------------------------

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "lowercasing",
			input:    "PostgreSQL GORM Database",
			expected: []string{"postgresql", "gorm", "database"},
		},
		{
			name:     "stop word removal",
			input:    "the domain is in a redemption period",
			expected: []string{"domain", "redemption", "period"},
		},
		{
			name:     "special character splitting",
			input:    "domain-os/internal/workflows",
			expected: []string{"domain", "os", "internal", "workflows"},
		},
		{
			name:     "mixed",
			input:    "What is the BM25 score for this query?",
			expected: []string{"what", "bm25", "score", "query"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.input)
			if tt.expected == nil {
				tt.expected = []string{}
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// TestExtractMarkdownFromTS — template literal extraction for *Doc.ts files.
// ---------------------------------------------------------------------------

func TestExtractMarkdownFromTS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard export",
			input:    "export const MY_DOC = `# Hello World\n\nSome content here.`;\n",
			expected: "# Hello World\n\nSome content here.",
		},
		{
			name:     "no template literal",
			input:    "export const MY_DOC = 'no backticks';",
			expected: "",
		},
		{
			name:     "empty template literal",
			input:    "export const MY_DOC = ``;",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMarkdownFromTS(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNewKnowledgeService_MissingManifest — graceful error for missing index.
// ---------------------------------------------------------------------------

func TestNewKnowledgeService_MissingManifest(t *testing.T) {
	dir := t.TempDir()

	_, err := NewKnowledgeService(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read corpus manifest")
}

// ---------------------------------------------------------------------------
// TestNewKnowledgeService_SkipsMissingFiles — missing files are warned, not fatal.
// ---------------------------------------------------------------------------

func TestNewKnowledgeService_SkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	manifest := `sources:
  root-docs:
    - docs/exists.md
    - docs/missing.md
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	doc := `# Exists

## Content

This document exists and has enough content words to pass the minimum
chunk threshold for indexing by the knowledge service BM25 engine.
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "exists.md"), []byte(doc), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	assert.Equal(t, 1, svc.DocCount(), "should load only the existing file")
	assert.GreaterOrEqual(t, svc.ChunkCount(), 1)
}

// ---------------------------------------------------------------------------
// TestNewKnowledgeService_TSDocExtraction — test with a TypeScript doc file.
// ---------------------------------------------------------------------------

func TestNewKnowledgeService_TSDocExtraction(t *testing.T) {
	dir := t.TempDir()

	docsDir := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))

	frontendDir := filepath.Join(dir, "frontend", "lib", "constants")
	require.NoError(t, os.MkdirAll(frontendDir, 0o755))

	manifest := `sources:
  reference-guides:
    glob: "frontend/lib/constants/*Doc.ts"
`
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.yaml"), []byte(manifest), 0o644))

	tsDoc := "export const TEST_DOC_MARKDOWN = `# Test Reference Guide\n\n## Overview\n\nThis reference guide documents the testing strategy and validation\npatterns used across the domain registry management platform.\n\n## Configuration\n\nConfiguration is managed through environment variables injected via\nDoppler secrets management with automatic rotation and audit logging.\n`;\n"
	require.NoError(t, os.WriteFile(filepath.Join(frontendDir, "testDoc.ts"), []byte(tsDoc), 0o644))

	svc, err := NewKnowledgeService(dir)
	require.NoError(t, err)

	assert.Equal(t, 1, svc.DocCount())
	assert.GreaterOrEqual(t, svc.ChunkCount(), 2)

	// Search for content from the TS doc
	results, err := svc.Search("Doppler secrets environment variables", 5)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Contains(t, results[0].DocPath, "testDoc.ts")
}

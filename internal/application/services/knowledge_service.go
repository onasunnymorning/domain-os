package services

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// BM25 parameters (standard Okapi BM25 defaults)
// ---------------------------------------------------------------------------

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// KnowledgeService implements interfaces.KnowledgeService using an in-process
// BM25 inverted index over markdown documentation chunks.
type KnowledgeService struct {
	chunks   []docChunk
	index    invertedIndex // term → posting list
	avgDL    float64       // average document (chunk) length for BM25
	docCount int           // number of source documents loaded
}

// docChunk is a single indexed unit of documentation.
type docChunk struct {
	DocPath   string
	Section   string
	Content   string
	TermFreqs map[string]int
	Length    int // total term count
}

// posting stores term frequency data per chunk.
type posting struct {
	ChunkIdx int
	TermFreq int
}

// invertedIndex maps terms to their posting lists.
type invertedIndex map[string][]posting

// corpusManifest mirrors the YAML structure of docs/index.yaml.
type corpusManifest struct {
	Sources map[string]any `yaml:"sources"`
}

// sourceWithGlob represents a source entry that uses a glob pattern.
type sourceWithGlob struct {
	Glob string `yaml:"glob"`
}

// ---------------------------------------------------------------------------
// Stop words — common English words excluded from indexing
// ---------------------------------------------------------------------------

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "in": true, "on": true, "at": true,
	"to": true, "for": true, "of": true, "with": true, "and": true,
	"or": true, "but": true, "not": true, "this": true, "that": true,
	"it": true, "by": true, "from": true, "as": true, "be": true,
	"has": true, "have": true, "had": true,
}

// headingRe matches markdown headings at level 2 (##) or 3 (###).
var headingRe = regexp.MustCompile(`^(#{2,3})\s+(.+)$`)

// tsTemplateLiteralRe matches the content inside a template literal
// exported from a TypeScript constant file: export const XYZ = `...`;
var tsTemplateLiteralRe = regexp.MustCompile("(?s)`([^`]+)`")

// tokenSplitRe splits text on non-alphanumeric characters.
var tokenSplitRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// NewKnowledgeService loads the corpus manifest from basePath/docs/index.yaml,
// reads and chunks all referenced documents, and builds a BM25 inverted index.
// Files that cannot be read are skipped with a warning log rather than failing
// the entire initialization.
func NewKnowledgeService(basePath string) (*KnowledgeService, error) {
	manifestPath := filepath.Join(basePath, "docs", "index.yaml")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("NewKnowledgeService: failed to read corpus manifest %q: %w — check that docs/index.yaml exists at the project root", manifestPath, err)
	}

	var manifest corpusManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("NewKnowledgeService: failed to parse corpus manifest %q: %w — ensure the YAML is valid", manifestPath, err)
	}

	// Collect all file paths from the manifest.
	var filePaths []string

	for sourceName, raw := range manifest.Sources {
		switch v := raw.(type) {
		case []any:
			// Explicit file list
			for _, item := range v {
				if s, ok := item.(string); ok {
					filePaths = append(filePaths, s)
				}
			}
		case map[string]any:
			// Glob pattern entry
			if globPattern, ok := v["glob"].(string); ok {
				matches, err := filepath.Glob(filepath.Join(basePath, globPattern))
				if err != nil {
					slog.Warn("KnowledgeService: invalid glob pattern, skipping source",
						slog.String("source", sourceName),
						slog.String("glob", globPattern),
						slog.String("error", err.Error()),
					)
					continue
				}
				for _, match := range matches {
					// Convert back to relative path for consistent DocPath
					rel, _ := filepath.Rel(basePath, match)
					filePaths = append(filePaths, rel)
				}
			}
		default:
			slog.Warn("KnowledgeService: unexpected source format, skipping",
				slog.String("source", sourceName),
			)
		}
	}

	// Read and chunk all files.
	var allChunks []docChunk
	docCount := 0

	for _, relPath := range filePaths {
		absPath := filepath.Join(basePath, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			slog.Warn("KnowledgeService: failed to read document, skipping",
				slog.String("path", relPath),
				slog.String("error", err.Error()),
			)
			continue
		}

		docCount++
		var text string

		// For *Doc.ts files, extract markdown from template literal.
		if strings.HasSuffix(relPath, "Doc.ts") {
			text = extractMarkdownFromTS(string(content))
			if text == "" {
				slog.Warn("KnowledgeService: no template literal found in TS file, skipping",
					slog.String("path", relPath),
				)
				continue
			}
		} else {
			text = string(content)
		}

		chunks := chunkMarkdown(text, relPath)
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		slog.Warn("KnowledgeService: no chunks indexed — answer_system_question tool will return no results",
			slog.Int("docs_found", docCount),
		)
	}

	// Build inverted index.
	idx := make(invertedIndex)
	totalLength := 0

	for i := range allChunks {
		chunk := &allChunks[i]
		tokens := tokenize(chunk.Content)
		chunk.Length = len(tokens)
		chunk.TermFreqs = make(map[string]int)
		totalLength += chunk.Length

		for _, token := range tokens {
			chunk.TermFreqs[token]++
		}

		for term, freq := range chunk.TermFreqs {
			idx[term] = append(idx[term], posting{
				ChunkIdx: i,
				TermFreq: freq,
			})
		}
	}

	avgDL := 0.0
	if len(allChunks) > 0 {
		avgDL = float64(totalLength) / float64(len(allChunks))
	}

	slog.Info("KnowledgeService initialized",
		slog.Int("docs", docCount),
		slog.Int("chunks", len(allChunks)),
		slog.Int("terms", len(idx)),
	)

	return &KnowledgeService{
		chunks:   allChunks,
		index:    idx,
		avgDL:    avgDL,
		docCount: docCount,
	}, nil
}

// ---------------------------------------------------------------------------
// Interface implementation
// ---------------------------------------------------------------------------

// Search returns the top-N most relevant chunks for the given query using
// BM25 scoring. Returns an empty slice (not an error) if no chunks match.
func (s *KnowledgeService) Search(query string, topN int) ([]interfaces.SearchResult, error) {
	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}

	N := float64(len(s.chunks))
	scores := make([]float64, len(s.chunks))

	for _, term := range queryTokens {
		postings, ok := s.index[term]
		if !ok {
			continue
		}

		df := float64(len(postings))
		// BM25 IDF: log((N - df + 0.5) / (df + 0.5) + 1)
		idf := math.Log((N-df+0.5)/(df+0.5) + 1)

		for _, p := range postings {
			dl := float64(s.chunks[p.ChunkIdx].Length)
			freq := float64(p.TermFreq)
			// BM25 TF normalization
			tfNorm := (freq * (bm25K1 + 1)) / (freq + bm25K1*(1-bm25B+bm25B*dl/s.avgDL))
			scores[p.ChunkIdx] += idf * tfNorm
		}
	}

	// Collect non-zero scores.
	type scored struct {
		idx   int
		score float64
	}

	var candidates []scored
	for i, sc := range scores {
		if sc > 0 {
			candidates = append(candidates, scored{i, sc})
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Sort by score descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Take top-N.
	if topN > len(candidates) {
		topN = len(candidates)
	}

	results := make([]interfaces.SearchResult, topN)
	for i := 0; i < topN; i++ {
		c := candidates[i]
		chunk := &s.chunks[c.idx]
		results[i] = interfaces.SearchResult{
			DocPath: chunk.DocPath,
			Section: chunk.Section,
			Content: chunk.Content,
			Score:   c.score,
		}
	}

	return results, nil
}

// DocCount returns the number of source documents loaded.
func (s *KnowledgeService) DocCount() int {
	return s.docCount
}

// ChunkCount returns the total number of indexed chunks.
func (s *KnowledgeService) ChunkCount() int {
	return len(s.chunks)
}

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

// tokenize lowercases the input, splits on non-alphanumeric characters, and
// removes common English stop words.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	parts := tokenSplitRe.Split(lower, -1)

	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || stopWords[p] {
			continue
		}
		tokens = append(tokens, p)
	}

	return tokens
}

// ---------------------------------------------------------------------------
// Markdown chunking
// ---------------------------------------------------------------------------

// chunkMarkdown splits markdown content into chunks based on ## and ###
// headings. Each chunk includes a heading trail (e.g. "Parent > Child") and
// all content until the next heading of the same or higher level. Chunks with
// fewer than 10 words (after tokenizing) are skipped.
func chunkMarkdown(content, docPath string) []docChunk {
	lines := strings.Split(content, "\n")

	type section struct {
		level   int
		heading string
		lines   []string
	}

	var sections []section
	current := section{
		level:   0,
		heading: filepath.Base(docPath),
	}

	for _, line := range lines {
		match := headingRe.FindStringSubmatch(line)
		if match != nil {
			// Save current section
			if len(current.lines) > 0 || current.level == 0 {
				sections = append(sections, current)
			}

			level := len(match[1]) // 2 for ##, 3 for ###
			heading := strings.TrimSpace(match[2])

			current = section{
				level:   level,
				heading: heading,
			}
		} else {
			current.lines = append(current.lines, line)
		}
	}
	// Don't forget the last section.
	sections = append(sections, current)

	// Build heading trails and chunks.
	var chunks []docChunk
	var parentHeading string // track the ## heading for ### trail building

	for _, sec := range sections {
		text := strings.Join(sec.lines, "\n")

		// Skip chunks with fewer than 10 words.
		words := tokenize(text)
		if len(words) < 10 {
			continue
		}

		// Build heading trail.
		var trail string
		switch {
		case sec.level == 0:
			trail = sec.heading
		case sec.level == 2:
			parentHeading = sec.heading
			trail = sec.heading
		case sec.level == 3:
			if parentHeading != "" {
				trail = parentHeading + " > " + sec.heading
			} else {
				trail = sec.heading
			}
		default:
			trail = sec.heading
		}

		chunks = append(chunks, docChunk{
			DocPath: docPath,
			Section: trail,
			Content: text,
		})
	}

	// If no chunks were created (no headings or all too short), treat entire
	// document as one chunk if it has enough words.
	if len(chunks) == 0 {
		words := tokenize(content)
		if len(words) >= 10 {
			chunks = append(chunks, docChunk{
				DocPath: docPath,
				Section: filepath.Base(docPath),
				Content: content,
			})
		}
	}

	return chunks
}

// ---------------------------------------------------------------------------
// TypeScript template literal extraction
// ---------------------------------------------------------------------------

// extractMarkdownFromTS extracts markdown content from a TypeScript file that
// exports a template literal constant (e.g. `export const fooDoc = \`...\`;`).
// Returns the content between the backticks, or empty string if not found.
func extractMarkdownFromTS(content string) string {
	match := tsTemplateLiteralRe.FindStringSubmatch(content)
	if match == nil || len(match) < 2 {
		return ""
	}
	return match[1]
}

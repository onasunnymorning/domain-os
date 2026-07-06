package interfaces

// KnowledgeService provides BM25-based document retrieval for the
// answer_system_question agent tool. Documents are loaded from the
// corpus manifest (docs/index.yaml) and chunked by markdown headings.
type KnowledgeService interface {
	// Search returns the top-N most relevant chunks for the given query,
	// scored using BM25. Returns an empty slice (not an error) if no
	// chunks match the query terms.
	Search(query string, topN int) ([]SearchResult, error)

	// DocCount returns the number of source documents loaded from the corpus manifest.
	DocCount() int

	// ChunkCount returns the total number of indexed chunks across all documents.
	ChunkCount() int
}

// SearchResult represents a single chunk matched by the knowledge service.
type SearchResult struct {
	DocPath string  // source file path relative to project root
	Section string  // heading trail, e.g. "Escrow Import > Step Breakdown"
	Content string  // raw markdown text of this chunk
	Score   float64 // BM25 relevance score
}

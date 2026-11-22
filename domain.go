package main

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID        uuid.UUID `json:"id"`
	Filename  string    `json:"filename"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DocInfo struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	CreatedAt  time.Time `json:"created_at"`
	ChunkCount int       `json:"chunk_count"`
}

type SearchResult struct {
	ID         string  `json:"id"`
	Content    string  `json:"content"`
	ChunkIndex int     `json:"chunk_index"`
	Filename   string  `json:"filename"`
	DocumentID string  `json:"document_id"`
	Similarity float32 `json:"similarity"`
}

type SearchInput struct {
	Query    string `json:"query" binding:"required"`
	Document string `json:"document"`
	Limit    int    `json:"limit"`
}

type QueryInput struct {
	Query           string  `json:"query" binding:"required"`
	Document        string  `json:"document"`
	TopK            int     `json:"top_k"`
	Generate        bool    `json:"generate"`
	Stream          bool    `json:"stream"`
	Think           *bool   `json:"think,omitempty"`
	MinSimilarity   float32 `json:"min_similarity"`
	IncludeContexts *bool   `json:"include_contexts"`
}

type QueryOutput struct {
	Query         string   `json:"query"`
	Contexts      []string `json:"contexts"`
	Sources       []string `json:"sources"`
	MaxSimilarity float32  `json:"max_similarity"`
	Answer        string   `json:"answer,omitempty"`
}

type DocumentRepository interface {
	DeleteDocumentByName(ctx context.Context, name string) error
	CreateDocument(ctx context.Context, id uuid.UUID, filename, content string) error
	InsertChunk(ctx context.Context, docID uuid.UUID, content string, chunkIdx int, embedding []float32) error
	ListDocuments(ctx context.Context) ([]DocInfo, error)
	SearchChunks(ctx context.Context, embedding []float32, documentFilter string, limit int) ([]SearchResult, error)
}

type DocumentUsecase interface {
	UploadAndIndex(ctx context.Context, originalFilename, customName, tempFilePath string) (*Document, int, error)
	ListDocuments(ctx context.Context) ([]DocInfo, error)
	Search(ctx context.Context, input SearchInput) ([]SearchResult, error)
	Query(ctx context.Context, input QueryInput) (*QueryOutput, error)
	QueryStream(ctx context.Context, input QueryInput, onMeta func(meta QueryOutput) error, onToken func(token string) error) error
}

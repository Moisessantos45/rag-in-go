package main

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDocumentRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresDocumentRepository(pool *pgxpool.Pool) *PostgresDocumentRepository {
	return &PostgresDocumentRepository{pool: pool}
}

func (r *PostgresDocumentRepository) DeleteDocumentByName(ctx context.Context, name string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM documents WHERE filename = $1", name)
	return err
}

func (r *PostgresDocumentRepository) CreateDocument(ctx context.Context, id uuid.UUID, filename, content string) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO documents (id, filename, content) VALUES ($1, $2, $3)",
		id, filename, content)
	return err
}

func (r *PostgresDocumentRepository) InsertChunk(ctx context.Context, docID uuid.UUID, content string, chunkIdx int, embedding []float32) error {
	vecStr := vectorToString(embedding)
	_, err := r.pool.Exec(ctx,
		"INSERT INTO chunks (document_id, content, chunk_index, embedding) VALUES ($1, $2, $3, $4)",
		docID, content, chunkIdx, vecStr)
	return err
}

func (r *PostgresDocumentRepository) ListDocuments(ctx context.Context) ([]DocInfo, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT d.id, d.filename, d.created_at, COUNT(c.id) as chunk_count
		 FROM documents d
		 LEFT JOIN chunks c ON d.id = c.document_id
		 GROUP BY d.id, d.filename, d.created_at
		 ORDER BY d.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []DocInfo{}
	for rows.Next() {
		var d DocInfo
		if err := rows.Scan(&d.ID, &d.Filename, &d.CreatedAt, &d.ChunkCount); err == nil {
			docs = append(docs, d)
		}
	}
	return docs, nil
}

func (r *PostgresDocumentRepository) SearchChunks(ctx context.Context, embedding []float32, documentFilter string, limit int) ([]SearchResult, error) {
	vecStr := vectorToString(embedding)
	var rows pgx.Rows
	var err error

	if documentFilter != "" {
		rows, err = r.pool.Query(ctx,
			`SELECT c.id, c.content, c.chunk_index, d.filename, d.id, 1 - (c.embedding <=> $1) as similarity
			 FROM chunks c
			 JOIN documents d ON c.document_id = d.id
			 WHERE (d.filename ILIKE '%' || $2 || '%' OR d.id::text = $2)
			 ORDER BY c.embedding <=> $1
			 LIMIT $3`,
			vecStr, documentFilter, limit)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT c.id, c.content, c.chunk_index, d.filename, d.id, 1 - (c.embedding <=> $1) as similarity
			 FROM chunks c
			 JOIN documents d ON c.document_id = d.id
			 ORDER BY c.embedding <=> $1
			 LIMIT $2`,
			vecStr, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []SearchResult{}
	for rows.Next() {
		var s SearchResult
		if err := rows.Scan(&s.ID, &s.Content, &s.ChunkIndex, &s.Filename, &s.DocumentID, &s.Similarity); err == nil {
			results = append(results, s)
		}
	}
	return results, nil
}

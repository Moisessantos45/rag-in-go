package main

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

func NewDatabase(ctx context.Context, databaseURL string) (*Database, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &Database{Pool: pool}, nil
}

func (db *Database) Close() {
	db.Pool.Close()
}

func InitSchema(ctx context.Context, db *Database) error {
	pool := db.Pool

	_, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS documents (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		filename VARCHAR(255) NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		chunk_index INTEGER NOT NULL,
		embedding vector(768),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(document_id, chunk_index)
	);

	CREATE INDEX IF NOT EXISTS chunks_embedding_idx
	ON chunks USING hnsw (embedding vector_cosine_ops)
	WITH (m = 16, ef_construction = 64);

	CREATE INDEX IF NOT EXISTS chunks_document_id_idx ON chunks (document_id);
	`

	_, err = pool.Exec(ctx, schema)
	return err
}

func vectorToString(vec []float32) string {
	var sb strings.Builder
	sb.Grow(len(vec)*10 + 2)
	sb.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}

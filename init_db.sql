-- Habilitar pgvector
CREATE EXTENSION IF NOT EXISTS vector;

-- Tabla de documentos
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tabla de chunks con embeddings
CREATE TABLE IF NOT EXISTS chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    embedding vector(768),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(document_id, chunk_index)
);

-- �ndice para b�squedas r�pidas
CREATE INDEX IF NOT EXISTS chunks_embedding_idx 
ON chunks USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

-- �ndice por documento
CREATE INDEX IF NOT EXISTS chunks_document_id_idx ON chunks (document_id);

-- Vista de estad�sticas
CREATE OR REPLACE VIEW document_stats AS
SELECT 
    d.id,
    d.filename,
    d.created_at,
    COUNT(c.id) as chunk_count,
    pg_size_pretty(SUM(octet_length(c.content))) as total_size
FROM documents d
LEFT JOIN chunks c ON d.id = c.document_id
GROUP BY d.id, d.filename, d.created_at
ORDER BY d.created_at DESC;

-- Permisos y propiedad para user_db
ALTER TABLE IF EXISTS documents OWNER TO user_db;
ALTER TABLE IF EXISTS chunks OWNER TO user_db;
ALTER VIEW IF EXISTS document_stats OWNER TO user_db;

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO user_db;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO user_db;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO user_db;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO user_db;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO user_db;

SELECT 'Base de datos RAG inicializada correctamente' as status;
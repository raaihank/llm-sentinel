-- LLM-Sentinel Database Initialization
-- PostgreSQL with pgvector extension for vector similarity search

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create database user if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'sentinel') THEN
        CREATE ROLE sentinel WITH LOGIN PASSWORD 'sentinel_pass';
    END IF;
END
$$;

-- Grant necessary permissions
GRANT ALL PRIVILEGES ON DATABASE llm_sentinel TO sentinel;
GRANT ALL ON SCHEMA public TO sentinel;

-- Create security_vectors table for storing attack patterns and embeddings
CREATE TABLE IF NOT EXISTS security_vectors (
    id BIGSERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    text_hash VARCHAR(64) NOT NULL UNIQUE,
    label_text VARCHAR(50) NOT NULL,
    label INTEGER NOT NULL CHECK (label IN (0, 1)),
    embedding vector(384) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_security_vectors_label ON security_vectors(label);
CREATE INDEX IF NOT EXISTS idx_security_vectors_label_text ON security_vectors(label_text);
CREATE INDEX IF NOT EXISTS idx_security_vectors_created_at ON security_vectors(created_at);
CREATE INDEX IF NOT EXISTS idx_security_vectors_text_hash ON security_vectors(text_hash);

-- Create vector similarity index using IVFFlat
-- This will be created after we have some data
-- CREATE INDEX IF NOT EXISTS idx_security_vectors_embedding ON security_vectors 
--     USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_security_vectors_updated_at 
    BEFORE UPDATE ON security_vectors 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create table for caching vector search results
CREATE TABLE IF NOT EXISTS vector_cache_stats (
    id BIGSERIAL PRIMARY KEY,
    cache_hits BIGINT DEFAULT 0,
    cache_misses BIGINT DEFAULT 0,
    total_searches BIGINT DEFAULT 0,
    avg_search_time_ms FLOAT DEFAULT 0,
    last_updated TIMESTAMP DEFAULT NOW()
);

-- Insert initial stats row
INSERT INTO vector_cache_stats (cache_hits, cache_misses, total_searches, avg_search_time_ms)
VALUES (0, 0, 0, 0)
ON CONFLICT DO NOTHING;

-- Create view for security analytics
CREATE OR REPLACE VIEW security_analytics AS
SELECT 
    label_text,
    label,
    COUNT(*) as pattern_count,
    MIN(created_at) as first_seen,
    MAX(created_at) as last_updated
FROM security_vectors 
GROUP BY label_text, label
ORDER BY pattern_count DESC;

-- Grant permissions to sentinel user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO sentinel;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO sentinel;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO sentinel;

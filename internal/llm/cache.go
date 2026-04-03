package llm

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"

	_ "github.com/mattn/go-sqlite3"
)

var ErrCacheMiss = errors.New("cache miss")

type CachedResponse struct {
	Model      string
	Response   string
	TokensUsed int
	LatencyMs  int
	Metadata   map[string]interface{}
}

type Cache struct {
	db *sql.DB
}

func NewCache(dbPath string) (*Cache, error) {
	if dbPath == "" {
		dbPath = ":memory:"
	}
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS llm_cache (
		key TEXT PRIMARY KEY,
		model TEXT,
		response TEXT NOT NULL,
		tokens_used INTEGER DEFAULT 0,
		latency_ms INTEGER DEFAULT 0,
		metadata TEXT,
		embedding TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}
	return &Cache{db: db}, nil
}

func (c *Cache) Get(key string) (*CachedResponse, error) {
	var model, response string
	var tokensUsed, latencyMs int
	var metadataStr sql.NullString
	err := c.db.QueryRow(
		"SELECT model, response, tokens_used, latency_ms, metadata FROM llm_cache WHERE key = ?", key,
	).Scan(&model, &response, &tokensUsed, &latencyMs, &metadataStr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	resp := &CachedResponse{
		Model: model, Response: response, TokensUsed: tokensUsed, LatencyMs: latencyMs,
	}
	if metadataStr.Valid {
		_ = json.Unmarshal([]byte(metadataStr.String), &resp.Metadata)
	}
	return resp, nil
}

func (c *Cache) Set(key string, resp *CachedResponse, embedding []float64) error {
	metaJSON, _ := json.Marshal(resp.Metadata)
	embJSON, _ := json.Marshal(embedding)
	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO llm_cache (key, model, response, tokens_used, latency_ms, metadata, embedding) VALUES (?, ?, ?, ?, ?, ?, ?)",
		key, resp.Model, resp.Response, resp.TokensUsed, resp.LatencyMs, string(metaJSON), string(embJSON),
	)
	return err
}

func (c *Cache) FindSimilar(embedding []float64, threshold float64) (*CachedResponse, error) {
	rows, err := c.db.Query("SELECT key, model, response, tokens_used, latency_ms, metadata, embedding FROM llm_cache WHERE embedding IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bestResp *CachedResponse
	bestSim := -1.0
	for rows.Next() {
		var key, model, response string
		var tokensUsed, latencyMs int
		var metadataStr, embStr sql.NullString
		if err := rows.Scan(&key, &model, &response, &tokensUsed, &latencyMs, &metadataStr, &embStr); err != nil {
			continue
		}
		if !embStr.Valid {
			continue
		}
		var stored []float64
		if err := json.Unmarshal([]byte(embStr.String), &stored); err != nil {
			continue
		}
		sim := cosineSimilarity(embedding, stored)
		if sim > bestSim {
			bestSim = sim
			resp := &CachedResponse{Model: model, Response: response, TokensUsed: tokensUsed, LatencyMs: latencyMs}
			if metadataStr.Valid {
				_ = json.Unmarshal([]byte(metadataStr.String), &resp.Metadata)
			}
			bestResp = resp
		}
	}
	if bestSim >= threshold && bestResp != nil {
		return bestResp, nil
	}
	return nil, ErrCacheMiss
}

func (c *Cache) Close() error { return c.db.Close() }

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

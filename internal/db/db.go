package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver

	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/models"
)

// Store wraps the database connection pool.
type Store struct {
	db *sql.DB
}

// New opens a connection pool and verifies the database is reachable.
func New(ctx context.Context, url string) (*Store, error) {
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{db: conn}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Migrate creates the table if it doesn't exist yet.
func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS failures (
			id             BIGSERIAL PRIMARY KEY,
			pipeline_name  TEXT NOT NULL,
			build_number   TEXT NOT NULL DEFAULT '',
			log_excerpt    TEXT NOT NULL,
			ai_explanation TEXT NOT NULL DEFAULT '',
			fix_suggestion TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_failures_created_at
			ON failures (created_at DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// InsertFailure saves a failure and fills in its ID and CreatedAt.
func (s *Store) InsertFailure(ctx context.Context, f *models.Failure) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO failures (pipeline_name, build_number, log_excerpt)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		f.PipelineName, f.BuildNumber, f.LogExcerpt,
	).Scan(&f.ID, &f.CreatedAt)
}

// ListFailures returns the most recent failures, newest first.
func (s *Store) ListFailures(ctx context.Context, limit int) ([]models.Failure, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, pipeline_name, build_number, log_excerpt,
		        ai_explanation, fix_suggestion, created_at
		 FROM failures
		 ORDER BY created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	failures := []models.Failure{}
	for rows.Next() {
		var f models.Failure
		if err := rows.Scan(&f.ID, &f.PipelineName, &f.BuildNumber, &f.LogExcerpt,
			&f.AIExplanation, &f.FixSuggestion, &f.CreatedAt); err != nil {
			return nil, err
		}
		failures = append(failures, f)
	}
	return failures, rows.Err()
}

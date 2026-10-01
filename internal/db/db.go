// Package db owns the Postgres connection pool and database migrations.
package db

import (
	"context"
	"embed"
	"fmt"
	"time"

	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open creates a connection pool.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// Teach pgx to encode/decode NUMERIC columns as decimal.Decimal.
		pgxdecimal.Register(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies migrations in the given direction: "up", "down", "status"
// or "version".
func Migrate(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	switch direction {
	case "up":
		return goose.UpContext(ctx, sqlDB, "migrations")
	case "down":
		return goose.DownContext(ctx, sqlDB, "migrations")
	case "status":
		return goose.StatusContext(ctx, sqlDB, "migrations")
	case "version":
		v, err := goose.GetDBVersionContext(ctx, sqlDB)
		if err != nil {
			return err
		}
		fmt.Printf("migration version: %d\n", v)
		return nil
	default:
		return fmt.Errorf("unknown migration direction %q (want up, down, status or version)", direction)
	}
}

package database

import (
	"context"
	"embed"
	"fmt"

	"issue-tracker/internal/database/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type Database struct {
	Pool    *pgxpool.Pool
	Queries *sqlc.Queries
}

var migrationsFS embed.FS

func NewDatabase(ctx context.Context, connString string) (*Database, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	db := &Database{
		Pool:    pool,
		Queries: sqlc.New(pool),
	}
	if err := db.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func (db *Database) Migrate() error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(db.Pool)
	defer sqlDB.Close()

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

func (db *Database) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

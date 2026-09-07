package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string, migrations embed.FS) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		pool.Close()
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		sql, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			pool.Close()
			return nil, fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
	}
	return &Store{Pool: pool}, nil
}
func (s *Store) Close() { s.Pool.Close() }

func (s *Store) EnsureInitialAdmin(ctx context.Context, username, password string) error {
	if username == "" {
		return nil
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE role='admin')").Scan(&exists); err != nil || exists {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO users(username,password_hash,real_name,avatar_url,role,status) VALUES($1,$2,$1,'', 'admin','active')`, username, string(hash))
	return err
}

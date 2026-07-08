// Package store persists panel clients and their traffic counters in SQLite
// (pure-Go modernc driver, so cross-compilation needs no CGO).
package store

import (
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/tanselxy/singbox/internal/model"
)

// ErrNotFound is returned when a client lookup fails.
var ErrNotFound = errors.New("client not found")

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // sqlite: serialize writes, avoids "database is locked"
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS clients (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,
    uuid               TEXT NOT NULL,
    password           TEXT NOT NULL,
    ss2022_key         TEXT NOT NULL,
    shadowtls_password TEXT NOT NULL,
    sub_token          TEXT NOT NULL UNIQUE,
    enabled            INTEGER NOT NULL DEFAULT 1,
    quota_bytes        INTEGER NOT NULL DEFAULT 0,
    created_at         INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS traffic (
    client_id  INTEGER PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    up_bytes   INTEGER NOT NULL DEFAULT 0,
    down_bytes INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0
);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// CreateClient inserts a client and returns it with its assigned ID.
func (s *Store) CreateClient(c model.Client) (model.Client, error) {
	res, err := s.db.Exec(
		`INSERT INTO clients (name, uuid, password, ss2022_key, shadowtls_password, sub_token, enabled, quota_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.UUID, c.Password, c.SS2022Key, c.ShadowTLSPassword, c.SubToken, boolInt(c.Enabled), c.QuotaBytes, c.CreatedAt,
	)
	if err != nil {
		return model.Client{}, fmt.Errorf("create client: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Client{}, err
	}
	c.ID = id
	if _, err := s.db.Exec(`INSERT INTO traffic (client_id) VALUES (?)`, id); err != nil {
		return model.Client{}, fmt.Errorf("init traffic row: %w", err)
	}
	return c, nil
}

const clientColumns = `id, name, uuid, password, ss2022_key, shadowtls_password, sub_token, enabled, quota_bytes, created_at`

// ListClients returns all clients ordered by id.
func (s *Store) ListClients() ([]model.Client, error) {
	rows, err := s.db.Query(`SELECT ` + clientColumns + ` FROM clients ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EnabledClients returns only enabled clients.
func (s *Store) EnabledClients() ([]model.Client, error) {
	all, err := s.ListClients()
	if err != nil {
		return nil, err
	}
	out := make([]model.Client, 0, len(all))
	for _, c := range all {
		if c.Enabled {
			out = append(out, c)
		}
	}
	return out, nil
}

// GetClient returns a client by id.
func (s *Store) GetClient(id int64) (model.Client, error) {
	row := s.db.QueryRow(`SELECT `+clientColumns+` FROM clients WHERE id = ?`, id)
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Client{}, ErrNotFound
	}
	return c, err
}

// GetClientByToken returns a client by its subscription token.
func (s *Store) GetClientByToken(token string) (model.Client, error) {
	row := s.db.QueryRow(`SELECT `+clientColumns+` FROM clients WHERE sub_token = ?`, token)
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Client{}, ErrNotFound
	}
	return c, err
}

// SetEnabled toggles a client's enabled flag.
func (s *Store) SetEnabled(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE clients SET enabled = ? WHERE id = ?`, boolInt(enabled), id)
	return err
}

// SetQuota updates a client's quota in bytes (0 = unlimited).
func (s *Store) SetQuota(id int64, quotaBytes int64) error {
	_, err := s.db.Exec(`UPDATE clients SET quota_bytes = ? WHERE id = ?`, quotaBytes, id)
	return err
}

// DeleteClient removes a client and its traffic row.
func (s *Store) DeleteClient(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM traffic WHERE client_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM clients WHERE id = ?`, id)
	return err
}

// Traffic is a client's cumulative traffic counters.
type Traffic struct {
	Up        int64
	Down      int64
	UpdatedAt int64
}

// AddTraffic accumulates upload/download bytes onto a client's counters.
func (s *Store) AddTraffic(clientID, up, down, at int64) error {
	_, err := s.db.Exec(
		`UPDATE traffic SET up_bytes = up_bytes + ?, down_bytes = down_bytes + ?, updated_at = ? WHERE client_id = ?`,
		up, down, at, clientID,
	)
	return err
}

// GetTraffic returns a client's cumulative counters.
func (s *Store) GetTraffic(clientID int64) (Traffic, error) {
	row := s.db.QueryRow(`SELECT up_bytes, down_bytes, updated_at FROM traffic WHERE client_id = ?`, clientID)
	var t Traffic
	if err := row.Scan(&t.Up, &t.Down, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Traffic{}, nil
		}
		return Traffic{}, err
	}
	return t, nil
}

// ResetTraffic zeroes a client's counters.
func (s *Store) ResetTraffic(clientID int64) error {
	_, err := s.db.Exec(`UPDATE traffic SET up_bytes = 0, down_bytes = 0 WHERE client_id = ?`, clientID)
	return err
}

type scanner interface{ Scan(dest ...any) error }

func scanClient(sc scanner) (model.Client, error) {
	var c model.Client
	var enabled int
	err := sc.Scan(&c.ID, &c.Name, &c.UUID, &c.Password, &c.SS2022Key,
		&c.ShadowTLSPassword, &c.SubToken, &enabled, &c.QuotaBytes, &c.CreatedAt)
	if err != nil {
		return model.Client{}, err
	}
	c.Enabled = enabled != 0
	return c, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

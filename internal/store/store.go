// Package store persists panel clients and their traffic counters in SQLite
// (pure-Go modernc driver, so cross-compilation needs no CGO).
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
    created_at         INTEGER NOT NULL,
    device_limit       INTEGER NOT NULL DEFAULT 0,
    expires_at         INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS traffic (
    client_id  INTEGER PRIMARY KEY REFERENCES clients(id) ON DELETE CASCADE,
    up_bytes   INTEGER NOT NULL DEFAULT 0,
    down_bytes INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS nodes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    tag         TEXT NOT NULL DEFAULT '',
    address     TEXT NOT NULL,
    token       TEXT NOT NULL,
    server_json TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    start_at    INTEGER NOT NULL DEFAULT 0,
    end_at      INTEGER NOT NULL DEFAULT 0,
    billing_cycle TEXT NOT NULL DEFAULT '',
    next_remind_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	// Add columns introduced after the initial schema, for databases created by
	// an earlier version. Duplicate-column errors are expected and ignored.
	for _, col := range []string{
		"ALTER TABLE clients ADD COLUMN device_limit INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE clients ADD COLUMN expires_at INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE nodes ADD COLUMN tag TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE nodes ADD COLUMN start_at INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE nodes ADD COLUMN end_at INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE nodes ADD COLUMN billing_cycle TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE nodes ADD COLUMN next_remind_at INTEGER NOT NULL DEFAULT 0",
	} {
		if _, err := s.db.Exec(col); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// GetSetting returns a persisted string setting.
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

// SetSetting persists a string setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// CreateClient inserts a client and returns it with its assigned ID.
func (s *Store) CreateClient(c model.Client) (model.Client, error) {
	res, err := s.db.Exec(
		`INSERT INTO clients (name, uuid, password, ss2022_key, shadowtls_password, sub_token, enabled, quota_bytes, created_at, device_limit, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.UUID, c.Password, c.SS2022Key, c.ShadowTLSPassword, c.SubToken, boolInt(c.Enabled), c.QuotaBytes, c.CreatedAt, c.DeviceLimit, c.ExpiresAt,
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

const clientColumns = `id, name, uuid, password, ss2022_key, shadowtls_password, sub_token, enabled, quota_bytes, created_at, device_limit, expires_at`

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

// ActiveClients returns clients that should currently be served: enabled and
// not past their expiry (evaluated against now, unix seconds).
func (s *Store) ActiveClients(now int64) ([]model.Client, error) {
	all, err := s.ListClients()
	if err != nil {
		return nil, err
	}
	out := make([]model.Client, 0, len(all))
	for _, c := range all {
		if c.Active(now) {
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

// UpdateClient updates editable client metadata while keeping credentials and
// accumulated traffic intact.
func (s *Store) UpdateClient(c model.Client) error {
	_, err := s.db.Exec(
		`UPDATE clients SET name = ?, quota_bytes = ?, device_limit = ?, expires_at = ? WHERE id = ?`,
		c.Name, c.QuotaBytes, c.DeviceLimit, c.ExpiresAt, c.ID,
	)
	return err
}

// UpdateClientCredentials replaces every client credential and subscription
// token while preserving metadata and traffic counters.
func (s *Store) UpdateClientCredentials(c model.Client) error {
	_, err := s.db.Exec(
		`UPDATE clients SET uuid = ?, password = ?, ss2022_key = ?, shadowtls_password = ?, sub_token = ? WHERE id = ?`,
		c.UUID, c.Password, c.SS2022Key, c.ShadowTLSPassword, c.SubToken, c.ID,
	)
	return err
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

// ---- nodes ----

// CreateNode inserts a remote node.
func (s *Store) CreateNode(n model.Node) (model.Node, error) {
	res, err := s.db.Exec(
		`INSERT INTO nodes (name, tag, address, token, server_json, created_at, start_at, end_at, billing_cycle, next_remind_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Name, n.Tag, n.Address, n.Token, n.ServerJSON, n.CreatedAt, n.StartAt, n.EndAt, n.BillingCycle, n.NextRemindAt,
	)
	if err != nil {
		return model.Node{}, fmt.Errorf("create node: %w", err)
	}
	n.ID, _ = res.LastInsertId()
	return n, nil
}

const nodeColumns = `id, name, tag, address, token, server_json, created_at, start_at, end_at, billing_cycle, next_remind_at`

// ListNodes returns all nodes ordered by id.
func (s *Store) ListNodes() ([]model.Node, error) {
	rows, err := s.db.Query(`SELECT ` + nodeColumns + ` FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Tag, &n.Address, &n.Token, &n.ServerJSON, &n.CreatedAt, &n.StartAt, &n.EndAt, &n.BillingCycle, &n.NextRemindAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNode returns a node by id.
func (s *Store) GetNode(id int64) (model.Node, error) {
	row := s.db.QueryRow(`SELECT `+nodeColumns+` FROM nodes WHERE id = ?`, id)
	var n model.Node
	err := row.Scan(&n.ID, &n.Name, &n.Tag, &n.Address, &n.Token, &n.ServerJSON, &n.CreatedAt, &n.StartAt, &n.EndAt, &n.BillingCycle, &n.NextRemindAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Node{}, ErrNotFound
	}
	return n, err
}

// UpdateNodeServer refreshes a node's cached server JSON.
func (s *Store) UpdateNodeServer(id int64, serverJSON string) error {
	_, err := s.db.Exec(`UPDATE nodes SET server_json = ? WHERE id = ?`, serverJSON, id)
	return err
}

// UpdateNodeDetails updates display metadata for a registered node.
func (s *Store) UpdateNodeDetails(id int64, name, tag string, startAt, endAt int64, billingCycle string) error {
	_, err := s.db.Exec(
		`UPDATE nodes SET name = ?, tag = ?, start_at = ?, end_at = ?, billing_cycle = ?, next_remind_at = 0 WHERE id = ?`,
		name, tag, startAt, endAt, billingCycle, id,
	)
	return err
}

// UpdateNodeBilling advances billing dates and the next reminder marker.
func (s *Store) UpdateNodeBilling(id int64, startAt, endAt, nextRemindAt int64) error {
	_, err := s.db.Exec(`UPDATE nodes SET start_at = ?, end_at = ?, next_remind_at = ? WHERE id = ?`, startAt, endAt, nextRemindAt, id)
	return err
}

// DeleteNode removes a node.
func (s *Store) DeleteNode(id int64) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
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
		&c.ShadowTLSPassword, &c.SubToken, &enabled, &c.QuotaBytes, &c.CreatedAt,
		&c.DeviceLimit, &c.ExpiresAt)
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

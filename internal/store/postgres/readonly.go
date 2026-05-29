package postgres

import (
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/config"
)

// ReplicaDB wraps primary + optional replica connections for read/write splitting.
type ReplicaDB struct {
	primary *sql.DB
	replica *sql.DB
}

// ConnectReplica connects to both primary and replica, falling back to primary if replica unavailable.
func ConnectReplica(cfg config.DatabaseConfig, replicaHost string) (*ReplicaDB, error) {
	primary, err := Connect(cfg)
	if err != nil {
		return nil, err
	}

	replicaCfg := cfg
	replicaCfg.Host = replicaHost
	replica, err := Connect(replicaCfg)
	if err != nil {
		replica = primary
	}

	return &ReplicaDB{primary: primary, replica: replica}, nil
}

func (db *ReplicaDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.replica.Query(query, args...)
}

func (db *ReplicaDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.replica.QueryRow(query, args...)
}

func (db *ReplicaDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.primary.Exec(query, args...)
}

func (db *ReplicaDB) Primary() *sql.DB { return db.primary }
func (db *ReplicaDB) Replica() *sql.DB { return db.replica }

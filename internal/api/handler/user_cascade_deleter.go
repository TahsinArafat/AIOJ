package handler

import (
	"context"
	"database/sql"
)

// UserCascadeDeleter deletes the users row; FK actions (000062) cascade or
// SET NULL dependents so a single DELETE is sufficient inside a transaction.
type UserCascadeDeleter struct {
	DB *sql.DB
}

// NewUserCascadeDeleter wires the deleter to the shared *sql.DB.
func NewUserCascadeDeleter(db *sql.DB) *UserCascadeDeleter {
	return &UserCascadeDeleter{DB: db}
}

// DeleteUserAndCascade removes the user row in a transaction.
func (d *UserCascadeDeleter) DeleteUserAndCascade(ctx context.Context, userID string) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

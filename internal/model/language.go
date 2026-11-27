package model

import (
	"encoding/json"
	"time"
)

// Language is one judge language config as stored in the `languages` table.
//
// Config is the LangConfig document (internal/judge/compiler/lang.go) held as
// JSONB, not a column per field: that struct is already the shape both the
// shipped lang/*.yaml file and the admin request body use, so storing it
// verbatim means a new field needs no migration. The store does not interpret
// it — it is decoded and validated in the handler and in the judge's loader.
//
// A soft-deleted language keeps its row. Seeding is INSERT-only so an operator's
// edit is never overwritten, and that makes a hard DELETE unsafe: the next boot
// would find the shipped key missing and seed it straight back. DeletedAt still
// occupies the primary key, so the seed's ON CONFLICT DO NOTHING leaves it gone.
type Language struct {
	Key       string          `json:"key"`
	Config    json.RawMessage `json:"config"`
	UpdatedAt time.Time       `json:"updated_at"`
	UpdatedBy *string         `json:"updated_by,omitempty"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

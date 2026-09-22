package model

import "time"

// ProblemI18n is a per-language translation of a problem's statement metadata.
//
// Ported from dmoj's i18n_translation model (Reference_Projects/OJ/judge/
// models/problem.py:87-95). Only the fields a solver actually reads are
// translated -- title and description. Slug, limits, and checker config are
// language-independent and stay on the problem itself.
//
// Title and Description are pointers so "not translated" is distinguishable
// from "translated to an empty string": nil falls back to the default.
type ProblemI18n struct {
	ProblemID   string    `json:"problem_id"`
	Language    string    `json:"language"`
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

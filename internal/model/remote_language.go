package model

type RemoteLanguage struct {
	ID                  string `json:"id"`
	Platform            string `json:"platform"`
	LocalID             string `json:"local_id"`
	RemoteID            string `json:"remote_id"`
	DisplayName         string `json:"display_name"`
	Enabled             bool   `json:"enabled"`
	SortOrder           int    `json:"sort_order"`
	InlineCommentPrefix string `json:"inline_comment_prefix"`
}

type RemoteLanguageConfig struct {
	Platform    string           `json:"platform"`
	Languages   []RemoteLanguage `json:"languages"`
}

package model

import "time"

type BlogPost struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	Title        string    `json:"title"`
	Content      string    `json:"content"`
	Tags         []string  `json:"tags"`
	Upvotes      int       `json:"upvotes"`
	Downvotes    int       `json:"downvotes"`
	CommentCount int       `json:"comment_count"`
	IsPinned     bool      `json:"is_pinned"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Comment struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	ParentType string    `json:"parent_type"`
	ParentID   string    `json:"parent_id"`
	Content    string    `json:"content"`
	Upvotes    int       `json:"upvotes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Vote struct {
	UserID     string    `json:"user_id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Value      int       `json:"value"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateBlogRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

type CreateCommentRequest struct {
	ParentType string `json:"parent_type"`
	ParentID   string `json:"parent_id"`
	Content    string `json:"content"`
}

type BlogListItem struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Title        string    `json:"title"`
	Tags         []string  `json:"tags"`
	Upvotes      int       `json:"upvotes"`
	CommentCount int       `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
}

package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

type BlogStore struct {
	db *sql.DB
}

func NewBlogStore(db *sql.DB) *BlogStore {
	return &BlogStore{db: db}
}

func (s *BlogStore) CreatePost(ctx context.Context, p *model.BlogPost) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO blog_posts (user_id, title, content, tags) VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`,
		p.UserID, p.Title, p.Content, pq.Array(p.Tags),
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (s *BlogStore) GetPostByID(ctx context.Context, id string) (*model.BlogPost, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var p model.BlogPost
	var tags []string
	err := s.db.QueryRowContext(ctx,
		`SELECT bp.id, bp.user_id, u.username, bp.title, bp.content, bp.tags, bp.upvotes, bp.downvotes,
		        bp.comment_count, bp.is_pinned, bp.created_at, bp.updated_at
		 FROM blog_posts bp JOIN users u ON bp.user_id = u.id WHERE bp.id = $1`,
		id).Scan(&p.ID, &p.UserID, &p.Username, &p.Title, &p.Content, pq.Array(&tags),
		&p.Upvotes, &p.Downvotes, &p.CommentCount, &p.IsPinned, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Tags = tags
	return &p, nil
}

func (s *BlogStore) ListPosts(ctx context.Context, offset, limit int, tag string) ([]model.BlogListItem, int, error) {
	var total int
	query := "SELECT COUNT(*) FROM blog_posts"
	args := []interface{}{}
	argIdx := 1

	if tag != "" {
		query += " WHERE $1 = ANY(tags)"
		args = append(args, tag)
		argIdx++
	}
	s.db.QueryRowContext(ctx, query, args...).Scan(&total)

	selectQuery := `SELECT bp.id, bp.user_id, u.username, bp.title, bp.tags, bp.upvotes, bp.comment_count, bp.created_at
	                FROM blog_posts bp JOIN users u ON bp.user_id = u.id`
	if tag != "" {
		selectQuery += " WHERE $1 = ANY(tags)"
	}
	selectQuery += fmt.Sprintf(" ORDER BY bp.is_pinned DESC, bp.created_at DESC OFFSET $%d LIMIT $%d", argIdx, argIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.BlogListItem
	for rows.Next() {
		var p model.BlogListItem
		var tagArr []string
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Title, pq.Array(&tagArr), &p.Upvotes, &p.CommentCount, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		p.Tags = tagArr
		items = append(items, p)
	}
	if items == nil {
		items = []model.BlogListItem{}
	}
	return items, total, nil
}

func (s *BlogStore) CreateComment(ctx context.Context, c *model.Comment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO comments (user_id, parent_type, parent_id, content) VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`,
		c.UserID, c.ParentType, c.ParentID, c.Content,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return err
	}

	if c.ParentType == "blog" {
		_, err = tx.ExecContext(ctx, "UPDATE blog_posts SET comment_count = comment_count + 1 WHERE id = $1", c.ParentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *BlogStore) GetComments(ctx context.Context, parentType, parentID string) ([]model.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, u.username, c.parent_type, c.parent_id, c.content, c.upvotes, c.created_at, c.updated_at
		 FROM comments c JOIN users u ON c.user_id = u.id
		 WHERE c.parent_type = $1 AND c.parent_id = $2 ORDER BY c.created_at`,
		parentType, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.UserID, &c.Username, &c.ParentType, &c.ParentID, &c.Content, &c.Upvotes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	if comments == nil {
		comments = []model.Comment{}
	}
	return comments, nil
}

func (s *BlogStore) Vote(ctx context.Context, userID, targetType, targetID string, value int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO votes (user_id, target_type, target_id, value) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, target_type, target_id) DO UPDATE SET value = $4`,
		userID, targetType, targetID, value)
	if err != nil {
		return err
	}

	if targetType == "blog" {
		_, err = tx.ExecContext(ctx, "UPDATE blog_posts SET upvotes = (SELECT COALESCE(SUM(CASE WHEN value = 1 THEN 1 ELSE 0 END), 0) FROM votes WHERE target_id = $1), downvotes = (SELECT COALESCE(SUM(CASE WHEN value = -1 THEN 1 ELSE 0 END), 0) FROM votes WHERE target_id = $1) WHERE id = $1", targetID)
		if err != nil {
			return err
		}
	} else if targetType == "comment" {
		_, err = tx.ExecContext(ctx, "UPDATE comments SET upvotes = (SELECT COALESCE(SUM(value), 0) FROM votes WHERE target_id = $1) WHERE id = $1", targetID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *BlogStore) GetUserVote(ctx context.Context, userID, targetType, targetID string) (int, error) {
	var value int
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM votes WHERE user_id = $1 AND target_type = $2 AND target_id = $3",
		userID, targetType, targetID).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return value, err
}

func (s *BlogStore) UpdatePost(ctx context.Context, id string, p *model.BlogPost) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE blog_posts SET title = $1, content = $2, tags = $3, is_pinned = $4, updated_at = NOW()
		 WHERE id = $5`,
		p.Title, p.Content, pq.Array(p.Tags), p.IsPinned, id)
	return err
}

func (s *BlogStore) DeletePost(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM blog_posts WHERE id = $1", id)
	return err
}

func (s *BlogStore) DeleteComment(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM comments WHERE id = $1", id)
	return err
}

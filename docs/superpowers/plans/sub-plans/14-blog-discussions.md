# Sub-Plan 14: Blog/Discussions

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to write blog posts, comment on problems/contests, and participate in discussions.

**Architecture:** Add `blog_posts` and `comments` tables, blog service, frontend blog and comment UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/model/blog.go` - Blog models
- `internal/store/postgres/blog.go` - Blog store
- `internal/api/handler/blog.go` - Blog handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add BlogStore interface
- `internal/api/router.go` - Add blog routes

### Frontend Files to Create
- `web/src/pages/BlogList.tsx` - Blog listing
- `web/src/pages/BlogDetail.tsx` - Blog post detail
- `web/src/pages/BlogCreate.tsx` - Create blog post
- `web/src/components/CommentSection.tsx` - Comment component

### Frontend Files to Modify
- `web/src/App.tsx` - Add blog routes
- `web/src/lib/api.ts` - Add blog API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000012_blog.up.sql`
- Create: `internal/store/migrations/000012_blog.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000012_blog.up.sql

CREATE TABLE blog_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}',
    upvotes INTEGER NOT NULL DEFAULT 0,
    downvotes INTEGER NOT NULL DEFAULT 0,
    comment_count INTEGER NOT NULL DEFAULT 0,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    parent_type VARCHAR(16) NOT NULL CHECK (parent_type IN ('blog', 'problem', 'contest')),
    parent_id UUID NOT NULL,
    content TEXT NOT NULL,
    upvotes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE votes (
    user_id UUID NOT NULL REFERENCES users(id),
    target_type VARCHAR(16) NOT NULL CHECK (target_type IN ('blog', 'comment')),
    target_id UUID NOT NULL,
    value SMALLINT NOT NULL CHECK (value IN (-1, 1)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, target_type, target_id)
);

CREATE INDEX idx_blog_posts_user ON blog_posts(user_id);
CREATE INDEX idx_blog_posts_tags ON blog_posts USING GIN(tags);
CREATE INDEX idx_comments_parent ON comments(parent_type, parent_id);
CREATE INDEX idx_votes_target ON votes(target_type, target_id);
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000012_blog.down.sql

DROP TABLE IF EXISTS votes;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS blog_posts;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000012_blog.*
git commit -m "feat(blog): add blog database migration"
```

---

### Task 2: Blog Models

**Files:**
- Create: `internal/model/blog.go`

- [ ] **Step 1: Create blog models**

```go
// internal/model/blog.go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/blog.go
git commit -m "feat(blog): add blog models"
```

---

### Task 3: Blog Store

**Files:**
- Create: `internal/store/postgres/blog.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add BlogStore interface**

```go
type BlogStore interface {
	CreatePost(ctx context.Context, p *model.BlogPost) error
	GetPostByID(ctx context.Context, id string) (*model.BlogPost, error)
	ListPosts(ctx context.Context, offset, limit int, tag string) ([]model.BlogListItem, int, error)
	UpdatePost(ctx context.Context, id string, p *model.BlogPost) error
	DeletePost(ctx context.Context, id string) error
	
	CreateComment(ctx context.Context, c *model.Comment) error
	GetComments(ctx context.Context, parentType, parentID string) ([]model.Comment, error)
	DeleteComment(ctx context.Context, id string) error
	
	Vote(ctx context.Context, userID, targetType, targetID string, value int) error
	GetUserVote(ctx context.Context, userID, targetType, targetID string) (int, error)
}
```

- [ ] **Step 2: Implement blog store**

```go
// internal/store/postgres/blog.go
package postgres

import (
	"context"
	"database/sql"

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
		p.UserID, p.Title, p.Content, p.Tags,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (s *BlogStore) GetPostByID(ctx context.Context, id string) (*model.BlogPost, error) {
	var p model.BlogPost
	err := s.db.QueryRowContext(ctx,
		`SELECT bp.id, bp.user_id, u.username, bp.title, bp.content, bp.tags, bp.upvotes, bp.downvotes,
		        bp.comment_count, bp.is_pinned, bp.created_at, bp.updated_at
		 FROM blog_posts bp JOIN users u ON bp.user_id = u.id WHERE bp.id = $1`,
		id).Scan(&p.ID, &p.UserID, &p.Username, &p.Title, &p.Content, &p.Tags,
		&p.Upvotes, &p.Downvotes, &p.CommentCount, &p.IsPinned, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *BlogStore) ListPosts(ctx context.Context, offset, limit int, tag string) ([]model.BlogListItem, int, error) {
	var total int
	query := "SELECT COUNT(*) FROM blog_posts"
	args := []interface{}{}
	
	if tag != "" {
		query += " WHERE $1 = ANY(tags)"
		args = append(args, tag)
	}
	s.db.QueryRowContext(ctx, query, args...).Scan(&total)
	
	selectQuery := `SELECT bp.id, bp.user_id, u.username, bp.title, bp.tags, bp.upvotes, bp.comment_count, bp.created_at
	                FROM blog_posts bp JOIN users u ON bp.user_id = u.id`
	if tag != "" {
		selectQuery += " WHERE $1 = ANY(tags)"
	}
	selectQuery += " ORDER BY bp.is_pinned DESC, bp.created_at DESC OFFSET $2 LIMIT $3"
	args = append(args, offset, limit)
	
	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.BlogListItem
	for rows.Next() {
		var p model.BlogListItem
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Title, &p.Tags, &p.Upvotes, &p.CommentCount, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
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
		`INSERT INTO comments (user_id, parent_type, parent_id, content) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		c.UserID, c.ParentType, c.ParentID, c.Content,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return err
	}
	
	// Update comment count for blog posts
	if c.ParentType == "blog" {
		tx.ExecContext(ctx, "UPDATE blog_posts SET comment_count = comment_count + 1 WHERE id = $1", c.ParentID)
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
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO votes (user_id, target_type, target_id, value) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, target_type, target_id) DO UPDATE SET value = $4`,
		userID, targetType, targetID, value)
	
	// Update vote counts
	if targetType == "blog" {
		s.db.ExecContext(ctx, "UPDATE blog_posts SET upvotes = (SELECT COUNT(*) FROM votes WHERE target_id = $1 AND value = 1), downvotes = (SELECT COUNT(*) FROM votes WHERE target_id = $1 AND value = -1) WHERE id = $1", targetID)
	} else if targetType == "comment" {
		s.db.ExecContext(ctx, "UPDATE comments SET upvotes = (SELECT COUNT(*) FROM votes WHERE target_id = $1 AND value = 1) WHERE id = $1", targetID)
	}
	
	return err
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
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/blog.go
git commit -m "feat(blog): add blog store"
```

---

### Task 4: Blog Handler and Frontend

**Files:**
- Create: `internal/api/handler/blog.go`
- Modify: `internal/api/router.go`
- Create: `web/src/pages/BlogList.tsx`
- Create: `web/src/pages/BlogDetail.tsx`
- Create: `web/src/components/CommentSection.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Create blog handler**

```go
// internal/api/handler/blog.go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type BlogHandler struct {
	store *postgres.BlogStore
}

func NewBlogHandler(s *postgres.BlogStore) *BlogHandler {
	return &BlogHandler{store: s}
}

func (h *BlogHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateBlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	p := &model.BlogPost{
		UserID:  claims.UserID,
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
	}
	
	if err := h.store.CreatePost(r.Context(), p); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, p)
}

func (h *BlogHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.store.GetPostByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *BlogHandler) ListPosts(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	tag := r.URL.Query().Get("tag")
	
	items, total, _ := h.store.ListPosts(r.Context(), offset, limit, tag)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *BlogHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	c := &model.Comment{
		UserID:     claims.UserID,
		ParentType: req.ParentType,
		ParentID:   req.ParentID,
		Content:    req.Content,
	}
	
	if err := h.store.CreateComment(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, c)
}

func (h *BlogHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	parentType := chi.URLParam(r, "type")
	parentID := chi.URLParam(r, "id")
	
	comments, err := h.store.GetComments(r.Context(), parentType, parentID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": comments})
}

func (h *BlogHandler) Vote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Value      int    `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	if err := h.store.Vote(r.Context(), claims.UserID, req.TargetType, req.TargetID, req.Value); err != nil {
		http.Error(w, "vote failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Add routes**

```go
r.Route("/api/blog", func(r chi.Router) {
	r.Get("/", blogH.ListPosts)
	r.Get("/{id}", blogH.GetPost)
	r.Get("/{type}/{id}/comments", blogH.GetComments)
	
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", blogH.CreatePost)
		r.Post("/comments", blogH.CreateComment)
		r.Post("/vote", blogH.Vote)
	})
})
```

- [ ] **Step 3: Add blog API calls**

```typescript
blog: {
    list: (offset = 0, limit = 20, tag?: string) => {
        let url = `/blog?offset=${offset}&limit=${limit}`;
        if (tag) url += `&tag=${tag}`;
        return request<{ data: any[]; total: number }>(url);
    },
    get: (id: string) => request<any>(`/blog/${id}`),
    create: (data: any) => request<any>('/blog', { method: 'POST', body: JSON.stringify(data) }),
    getComments: (type: string, id: string) => request<any>(`/blog/${type}/${id}/comments`),
    createComment: (data: any) => request<any>('/blog/comments', { method: 'POST', body: JSON.stringify(data) }),
    vote: (data: { target_type: string; target_id: string; value: number }) =>
        request('/blog/vote', { method: 'POST', body: JSON.stringify(data) }),
},
```

- [ ] **Step 4: Create CommentSection component**

```tsx
// web/src/components/CommentSection.tsx
import { useState, useEffect } from 'react';
import { api } from '../lib/api';

interface CommentSectionProps {
  parentType: string;
  parentId: string;
}

export default function CommentSection({ parentType, parentId }: CommentSectionProps) {
  const [comments, setComments] = useState<any[]>([]);
  const [newComment, setNewComment] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.blog.getComments(parentType, parentId)
      .then(d => setComments(d.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [parentType, parentId]);

  const handleSubmit = async () => {
    if (!newComment.trim()) return;
    
    try {
      const comment = await api.blog.createComment({
        parent_type: parentType,
        parent_id: parentId,
        content: newComment,
      });
      setComments([...comments, comment]);
      setNewComment('');
    } catch (e: any) {
      alert('Failed: ' + e.message);
    }
  };

  return (
    <div className="space-y-4">
      <h3 className="font-semibold">Comments ({comments.length})</h3>
      
      {/* Comment Input */}
      <div className="flex gap-2">
        <textarea
          value={newComment}
          onChange={(e) => setNewComment(e.target.value)}
          placeholder="Write a comment..."
          className="flex-1 border rounded px-3 py-2 text-sm"
          rows={2}
        />
        <button
          onClick={handleSubmit}
          disabled={!newComment.trim()}
          className="bg-blue-600 text-white px-4 py-2 rounded text-sm disabled:opacity-50"
        >
          Post
        </button>
      </div>
      
      {/* Comments List */}
      {loading ? (
        <p className="text-gray-500">Loading comments...</p>
      ) : (
        <div className="space-y-3">
          {comments.map(c => (
            <div key={c.id} className="border rounded p-3">
              <div className="flex items-center justify-between mb-2">
                <span className="font-medium text-sm">{c.username}</span>
                <span className="text-xs text-gray-500">
                  {new Date(c.created_at).toLocaleDateString()}
                </span>
              </div>
              <p className="text-sm text-gray-700">{c.content}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 5: Create BlogList page**

```tsx
// web/src/pages/BlogList.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';

export default function BlogList() {
  const [posts, setPosts] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  useEffect(() => {
    api.blog.list(offset, limit).then(d => {
      setPosts(d.data || []);
      setTotal(d.total || 0);
    }).catch(console.error);
  }, [offset]);

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Blog</h1>
        <Link to="/blog/create" className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700">
          Write Post
        </Link>
      </div>

      <div className="space-y-4">
        {posts.map(p => (
          <Link key={p.id} to={`/blog/${p.id}`} className="block">
            <div className="border rounded-lg p-4 hover:bg-gray-50">
              <h3 className="font-semibold">{p.title}</h3>
              <div className="flex items-center gap-4 mt-2 text-sm text-gray-500">
                <span>{p.username}</span>
                <span>{p.upvotes} votes</span>
                <span>{p.comment_count} comments</span>
                <span>{new Date(p.created_at).toLocaleDateString()}</span>
              </div>
              {p.tags?.length > 0 && (
                <div className="flex gap-2 mt-2">
                  {p.tags.map((tag: string) => (
                    <span key={tag} className="text-xs bg-gray-100 px-2 py-0.5 rounded">
                      {tag}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Create BlogDetail page**

```tsx
// web/src/pages/BlogDetail.tsx
import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../lib/api';
import CommentSection from '../components/CommentSection';

export default function BlogDetail() {
  const { id } = useParams<{ id: string }>();
  const [post, setPost] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    api.blog.get(id).then(setPost).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  const handleVote = async (value: number) => {
    if (!id) return;
    await api.blog.vote({ target_type: 'blog', target_id: id, value });
    setPost({ ...post, upvotes: post.upvotes + value });
  };

  if (loading) return <div>Loading...</div>;
  if (!post) return <div>Post not found</div>;

  return (
    <div className="max-w-3xl mx-auto">
      <article className="mb-8">
        <h1 className="text-3xl font-bold mb-4">{post.title}</h1>
        <div className="flex items-center gap-4 text-sm text-gray-500 mb-6">
          <span>{post.username}</span>
          <span>{new Date(post.created_at).toLocaleDateString()}</span>
        </div>
        <div className="prose max-w-none">{post.content}</div>
        
        {/* Voting */}
        <div className="flex items-center gap-4 mt-6 pt-6 border-t">
          <button onClick={() => handleVote(1)} className="text-gray-500 hover:text-green-600">
            ▲
          </button>
          <span className="font-medium">{post.upvotes}</span>
          <button onClick={() => handleVote(-1)} className="text-gray-500 hover:text-red-600">
            ▼
          </button>
        </div>
      </article>

      {/* Comments */}
      <CommentSection parentType="blog" parentId={id!} />
    </div>
  );
}
```

- [ ] **Step 7: Add routes**

```tsx
<Route path="/blog" element={<BlogList />} />
<Route path="/blog/create" element={<BlogCreate />} />
<Route path="/blog/:id" element={<BlogDetail />} />
```

- [ ] **Step 8: Commit**

```bash
git add internal/api/handler/blog.go internal/api/router.go web/src/pages/Blog*.tsx web/src/components/CommentSection.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(blog): add blog and comments functionality"
```

---

## Verification Checklist

- [ ] Blog posts can be created
- [ ] Blog list displays correctly
- [ ] Comments can be added
- [ ] Voting works (up/down)
- [ ] Comment count updates
- [ ] Tags filter works

---

## Notes

1. **Markdown**: Content supports Markdown formatting
2. **Tags**: Optional tags for categorization
3. **Voting**: One vote per user per item
4. **Comments**: Nested under blog posts, problems, or contests

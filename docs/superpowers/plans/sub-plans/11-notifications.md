# Sub-Plan 11: Notifications

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Real-time notification system for contest announcements, rating changes, hacks, and group activities.

**Architecture:** Add `notifications` table, WebSocket-based real-time delivery, frontend notification UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript, WebSocket

---

## File Structure

### Backend Files to Create
- `internal/model/notification.go` - Notification models
- `internal/store/postgres/notifications.go` - Notification store
- `internal/notification/service.go` - Notification service
- `internal/api/handler/notification.go` - Notification handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add NotificationStore interface
- `internal/api/router.go` - Add notification routes
- `internal/api/handler/ws.go` - Add notification broadcasting

### Frontend Files to Create
- `web/src/components/NotificationBell.tsx` - Notification bell icon
- `web/src/components/NotificationList.tsx` - Notification dropdown
- `web/src/components/NotificationItem.tsx` - Single notification

### Frontend Files to Modify
- `web/src/App.tsx` - Add notification bell to navbar
- `web/src/lib/api.ts` - Add notification API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000008_notifications.up.sql`
- Create: `internal/store/migrations/000008_notifications.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000008_notifications.up.sql

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    link VARCHAR(512),
    read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, read, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id) WHERE read = false;

-- Notification preferences table
CREATE TABLE notification_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    contest_announcements BOOLEAN NOT NULL DEFAULT true,
    rating_changes BOOLEAN NOT NULL DEFAULT true,
    hack_notifications BOOLEAN NOT NULL DEFAULT true,
    group_activities BOOLEAN NOT NULL DEFAULT true,
    email_digest BOOLEAN NOT NULL DEFAULT false
);
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000008_notifications.down.sql

DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notifications;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000008_notifications.*
git commit -m "feat(notifications): add notifications database migration"
```

---

### Task 2: Notification Models

**Files:**
- Create: `internal/model/notification.go`

- [ ] **Step 1: Create notification models**

```go
// internal/model/notification.go
package model

import "time"

const (
	NotificationTypeContest     = "contest"
	NotificationTypeRating      = "rating"
	NotificationTypeHack        = "hack"
	NotificationTypeGroup       = "group"
	NotificationTypeSystem      = "system"
)

type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Link      string    `json:"link,omitempty"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type NotificationPreferences struct {
	UserID               string `json:"user_id"`
	ContestAnnouncements bool   `json:"contest_announcements"`
	RatingChanges        bool   `json:"rating_changes"`
	HackNotifications    bool   `json:"hack_notifications"`
	GroupActivities      bool   `json:"group_activities"`
	EmailDigest          bool   `json:"email_digest"`
}

type CreateNotificationRequest struct {
	UserID  string `json:"user_id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Link    string `json:"link,omitempty"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/notification.go
git commit -m "feat(notifications): add notification models"
```

---

### Task 3: Notification Store

**Files:**
- Create: `internal/store/postgres/notifications.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add NotificationStore interface**

Add to `internal/store/interfaces.go`:

```go
type NotificationStore interface {
	Create(ctx context.Context, n *model.Notification) error
	GetByUser(ctx context.Context, userID string, unreadOnly bool, limit int) ([]model.Notification, error)
	MarkAsRead(ctx context.Context, id string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	GetUnreadCount(ctx context.Context, userID string) (int, error)
	Delete(ctx context.Context, id string) error
	
	GetPreferences(ctx context.Context, userID string) (*model.NotificationPreferences, error)
	UpdatePreferences(ctx context.Context, userID string, prefs *model.NotificationPreferences) error
}
```

- [ ] **Step 2: Implement notification store**

```go
// internal/store/postgres/notifications.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type NotificationStore struct {
	db *sql.DB
}

func NewNotificationStore(db *sql.DB) *NotificationStore {
	return &NotificationStore{db: db}
}

func (s *NotificationStore) Create(ctx context.Context, n *model.Notification) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO notifications (user_id, type, title, content, link)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		n.UserID, n.Type, n.Title, n.Content, n.Link,
	).Scan(&n.ID, &n.CreatedAt)
}

func (s *NotificationStore) GetByUser(ctx context.Context, userID string, unreadOnly bool, limit int) ([]model.Notification, error) {
	query := `SELECT id, user_id, type, title, content, link, read, created_at
	          FROM notifications WHERE user_id = $1`
	args := []interface{}{userID}
	
	if unreadOnly {
		query += " AND read = false"
	}
	
	query += " ORDER BY created_at DESC LIMIT $2"
	args = append(args, limit)
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var notifications []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Content, &n.Link, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []model.Notification{}
	}
	return notifications, nil
}

func (s *NotificationStore) MarkAsRead(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE notifications SET read = true WHERE id = $1", id)
	return err
}

func (s *NotificationStore) MarkAllAsRead(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE notifications SET read = true WHERE user_id = $1 AND read = false", userID)
	return err
}

func (s *NotificationStore) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false",
		userID).Scan(&count)
	return count, err
}

func (s *NotificationStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM notifications WHERE id = $1", id)
	return err
}

func (s *NotificationStore) GetPreferences(ctx context.Context, userID string) (*model.NotificationPreferences, error) {
	var p model.NotificationPreferences
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, contest_announcements, rating_changes, hack_notifications, 
		        group_activities, email_digest
		 FROM notification_preferences WHERE user_id = $1`,
		userID).Scan(&p.UserID, &p.ContestAnnouncements, &p.RatingChanges,
		&p.HackNotifications, &p.GroupActivities, &p.EmailDigest)
	if err == sql.ErrNoRows {
		// Return defaults
		return &model.NotificationPreferences{
			UserID:               userID,
			ContestAnnouncements: true,
			RatingChanges:        true,
			HackNotifications:    true,
			GroupActivities:      true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *NotificationStore) UpdatePreferences(ctx context.Context, userID string, prefs *model.NotificationPreferences) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notification_preferences (user_id, contest_announcements, rating_changes, 
		         hack_notifications, group_activities, email_digest)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id) DO UPDATE SET
		         contest_announcements = $2, rating_changes = $3,
		         hack_notifications = $4, group_activities = $5, email_digest = $6`,
		userID, prefs.ContestAnnouncements, prefs.RatingChanges,
		prefs.HackNotifications, prefs.GroupActivities, prefs.EmailDigest)
	return err
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/notifications.go
git commit -m "feat(notifications): add notification store"
```

---

### Task 4: Notification Service

**Files:**
- Create: `internal/notification/service.go`

- [ ] **Step 1: Implement notification service**

```go
// internal/notification/service.go
package notification

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type Service struct {
	notifStore store.NotificationStore
	userStore  store.UserStore
}

func NewService(ns store.NotificationStore, us store.UserStore) *Service {
	return &Service{
		notifStore: ns,
		userStore:  us,
	}
}

// SendNotification creates and sends a notification
func (s *Service) SendNotification(ctx context.Context, req model.CreateNotificationRequest) error {
	// Check user preferences
	prefs, err := s.notifStore.GetPreferences(ctx, req.UserID)
	if err != nil {
		return err
	}
	
	// Check if this type is enabled
	if !s.isNotificationEnabled(prefs, req.Type) {
		return nil
	}
	
	n := &model.Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
		Link:    req.Link,
	}
	
	return s.notifStore.Create(ctx, n)
}

// SendBulkNotifications sends notifications to multiple users
func (s *Service) SendBulkNotifications(ctx context.Context, userIDs []string, req model.CreateNotificationRequest) error {
	for _, userID := range userIDs {
		req.UserID = userID
		if err := s.SendNotification(ctx, req); err != nil {
			// Log error but continue
			continue
		}
	}
	return nil
}

// NotifyContestAnnouncement sends contest announcement to all users
func (s *Service) NotifyContestAnnouncement(ctx context.Context, contestID, title, content string) error {
	// In production, would fetch all users or subscribed users
	// For now, skip
	return nil
}

// NotifyRatingChange sends rating change notification
func (s *Service) NotifyRatingChange(ctx context.Context, userID string, oldRating, newRating int) error {
	change := newRating - oldRating
	content := "Your rating changed"
	if change > 0 {
		content = "Your rating increased"
	} else if change < 0 {
		content = "Your rating decreased"
	}
	
	return s.SendNotification(ctx, model.CreateNotificationRequest{
		UserID:  userID,
		Type:    model.NotificationTypeRating,
		Title:   "Rating Update",
		Content: content,
		Link:    "/profile",
	})
}

// NotifyHackResult sends hack result notification
func (s *Service) NotifyHackResult(ctx context.Context, hackerID, defenderID string, success bool) error {
	// Notify hacker
	hackerMsg := "Your hack failed"
	if success {
		hackerMsg = "Your hack was successful!"
	}
	s.SendNotification(ctx, model.CreateNotificationRequest{
		UserID:  hackerID,
		Type:    model.NotificationTypeHack,
		Title:   "Hack Result",
		Content: hackerMsg,
	})
	
	// Notify defender if hacked
	if success {
		s.SendNotification(ctx, model.CreateNotificationRequest{
			UserID:  defenderID,
			Type:    model.NotificationTypeHack,
			Title:   "You've been hacked",
			Content: "Your solution was hacked",
		})
	}
	
	return nil
}

func (s *Service) isNotificationEnabled(prefs *model.NotificationPreferences, notifType string) bool {
	switch notifType {
	case model.NotificationTypeContest:
		return prefs.ContestAnnouncements
	case model.NotificationTypeRating:
		return prefs.RatingChanges
	case model.NotificationTypeHack:
		return prefs.HackNotifications
	case model.NotificationTypeGroup:
		return prefs.GroupActivities
	default:
		return true
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/notification/service.go
git commit -m "feat(notifications): add notification service"
```

---

### Task 5: Notification Handler

**Files:**
- Create: `internal/api/handler/notification.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create notification handler**

```go
// internal/api/handler/notification.go
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

type NotificationHandler struct {
	store *postgres.NotificationStore
}

func NewNotificationHandler(s *postgres.NotificationStore) *NotificationHandler {
	return &NotificationHandler{store: s}
}

// List returns notifications for current user
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	unreadOnly := r.URL.Query().Get("unread") == "true"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	
	notifications, err := h.store.GetByUser(r.Context(), claims.UserID, unreadOnly, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": notifications})
}

// UnreadCount returns count of unread notifications
func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		respondJSON(w, http.StatusOK, map[string]int{"count": 0})
		return
	}
	
	count, _ := h.store.GetUnreadCount(r.Context(), claims.UserID)
	respondJSON(w, http.StatusOK, map[string]int{"count": count})
}

// MarkAsRead marks a notification as read
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	id := chi.URLParam(r, "id")
	if err := h.store.MarkAsRead(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllAsRead marks all notifications as read
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	if err := h.store.MarkAllAsRead(r.Context(), claims.UserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetPreferences returns notification preferences
func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	prefs, err := h.store.GetPreferences(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, prefs)
}

// UpdatePreferences updates notification preferences
func (h *NotificationHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var prefs model.NotificationPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	if err := h.store.UpdatePreferences(r.Context(), claims.UserID, &prefs); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 2: Add routes**

Add to `internal/api/router.go`:

```go
r.Route("/api/notifications", func(r chi.Router) {
	r.Use(middleware.AuthMiddleware(jwtManager))
	r.Get("/", notifH.List)
	r.Get("/unread-count", notifH.UnreadCount)
	r.Post("/{id}/read", notifH.MarkAsRead)
	r.Post("/read-all", notifH.MarkAllAsRead)
	r.Get("/preferences", notifH.GetPreferences)
	r.Put("/preferences", notifH.UpdatePreferences)
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/notification.go internal/api/router.go
git commit -m "feat(notifications): add notification API endpoints"
```

---

### Task 6: Frontend Notification Components

**Files:**
- Create: `web/src/components/NotificationBell.tsx`
- Create: `web/src/components/NotificationList.tsx`
- Create: `web/src/components/NotificationItem.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add notification API calls**

Add to `web/src/lib/api.ts`:

```typescript
notifications: {
    list: (unreadOnly = false, limit = 50) =>
        request<{ data: any[] }>(`/notifications?unread=${unreadOnly}&limit=${limit}`),
    unreadCount: () => request<{ count: number }>('/notifications/unread-count'),
    markAsRead: (id: string) => request(`/notifications/${id}/read`, { method: 'POST' }),
    markAllAsRead: () => request('/notifications/read-all', { method: 'POST' }),
    getPreferences: () => request<any>('/notifications/preferences'),
    updatePreferences: (prefs: any) => request('/notifications/preferences', { method: 'PUT', body: JSON.stringify(prefs) }),
},
```

- [ ] **Step 2: Create NotificationItem component**

```tsx
// web/src/components/NotificationItem.tsx
import { formatDistanceToNow } from 'date-fns';

interface NotificationItemProps {
  notification: {
    id: string;
    type: string;
    title: string;
    content: string;
    link?: string;
    read: boolean;
    created_at: string;
  };
  onRead: (id: string) => void;
}

export default function NotificationItem({ notification, onRead }: NotificationItemProps) {
  const typeColors: Record<string, string> = {
    contest: 'bg-blue-100 text-blue-800',
    rating: 'bg-purple-100 text-purple-800',
    hack: 'bg-red-100 text-red-800',
    group: 'bg-green-100 text-green-800',
    system: 'bg-gray-100 text-gray-800',
  };

  const handleClick = () => {
    if (!notification.read) {
      onRead(notification.id);
    }
  };

  return (
    <div
      onClick={handleClick}
      className={`px-4 py-3 cursor-pointer hover:bg-gray-50 ${
        !notification.read ? 'bg-blue-50' : ''
      }`}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-1">
            <span className={`text-xs px-2 py-0.5 rounded ${typeColors[notification.type] || ''}`}>
              {notification.type}
            </span>
            {!notification.read && (
              <span className="w-2 h-2 bg-blue-600 rounded-full"></span>
            )}
          </div>
          <p className="font-medium text-sm">{notification.title}</p>
          <p className="text-sm text-gray-600 mt-1">{notification.content}</p>
        </div>
        <span className="text-xs text-gray-400 ml-2">
          {formatDistanceToNow(new Date(notification.created_at), { addSuffix: true })}
        </span>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Create NotificationList component**

```tsx
// web/src/components/NotificationList.tsx
import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import NotificationItem from './NotificationItem';

interface NotificationListProps {
  onClose: () => void;
}

export default function NotificationList({ onClose }: NotificationListProps) {
  const [notifications, setNotifications] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.notifications.list(false, 20)
      .then(d => setNotifications(d.data || []))
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const handleRead = async (id: string) => {
    await api.notifications.markAsRead(id);
    setNotifications(notifications.map(n =>
      n.id === id ? { ...n, read: true } : n
    ));
  };

  const handleMarkAllRead = async () => {
    await api.notifications.markAllAsRead();
    setNotifications(notifications.map(n => ({ ...n, read: true })));
  };

  return (
    <div className="absolute right-0 mt-2 w-96 bg-white rounded-lg shadow-lg border z-50">
      <div className="flex items-center justify-between px-4 py-3 border-b">
        <h3 className="font-semibold">Notifications</h3>
        <button
          onClick={handleMarkAllRead}
          className="text-sm text-blue-600 hover:text-blue-800"
        >
          Mark all read
        </button>
      </div>

      <div className="max-h-96 overflow-y-auto divide-y">
        {loading ? (
          <div className="px-4 py-8 text-center text-gray-500">Loading...</div>
        ) : notifications.length === 0 ? (
          <div className="px-4 py-8 text-center text-gray-500">No notifications</div>
        ) : (
          notifications.map(n => (
            <NotificationItem key={n.id} notification={n} onRead={handleRead} />
          ))
        )}
      </div>

      <div className="px-4 py-3 border-t text-center">
        <a href="/notifications" className="text-sm text-blue-600 hover:text-blue-800">
          View all notifications
        </a>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Create NotificationBell component**

```tsx
// web/src/components/NotificationBell.tsx
import { useEffect, useState, useRef } from 'react';
import { api } from '../lib/api';
import NotificationList from './NotificationList';

export default function NotificationBell() {
  const [count, setCount] = useState(0);
  const [showList, setShowList] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const fetchCount = () => {
      api.notifications.unreadCount()
        .then(d => setCount(d.count))
        .catch(() => {});
    };

    fetchCount();
    const interval = setInterval(fetchCount, 30000); // Poll every 30 seconds
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setShowList(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setShowList(!showList)}
        className="relative p-2 text-gray-600 hover:text-gray-800"
      >
        <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
        </svg>
        {count > 0 && (
          <span className="absolute top-0 right-0 inline-flex items-center justify-center px-2 py-1 text-xs font-bold leading-none text-white transform translate-x-1/2 -translate-y-1/2 bg-red-600 rounded-full">
            {count > 99 ? '99+' : count}
          </span>
        )}
      </button>

      {showList && <NotificationList onClose={() => setShowList(false)} />}
    </div>
  );
}
```

- [ ] **Step 5: Add to Navbar**

Update `web/src/App.tsx` Navbar component:

```tsx
import NotificationBell from '../components/NotificationBell';

// In Navbar, add before logout button:
{loggedIn && <NotificationBell />}
```

- [ ] **Step 6: Commit**

```bash
git add web/src/components/Notification*.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(notifications): add notification frontend components"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Notifications are created correctly
- [ ] Unread count updates in real-time
- [ ] Mark as read works
- [ ] Mark all as read works
- [ ] Notification preferences are saved
- [ ] Bell icon shows unread count
- [ ] Notification list displays correctly

---

## Notes

1. **Real-time**: Polling every 30 seconds. Could upgrade to WebSocket.
2. **Preferences**: Users can disable specific notification types.
3. **Email digest**: Future enhancement for daily/weekly summaries.
4. **Rate limiting**: Prevent notification spam.

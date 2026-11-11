// Package notify is the producer side of notifications.
//
// The bell, the unread count and the preferences page all worked from the day
// they shipped, against a table nothing ever wrote to: NotificationStore.Create
// had zero callers, so every user's inbox was permanently empty. This package is
// the missing half — one entry point that writes a notification, honouring the
// preferences the user already had a UI for.
package notify

import (
	"context"
	"log/slog"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Store is the slice of the notification store this package needs.
type Store interface {
	Create(ctx context.Context, n *model.Notification) error
	GetPreferences(ctx context.Context, userID string) (*model.NotificationPreferences, error)
}

// Category selects which preference flag governs a notification.
type Category string

const (
	CategoryContest Category = "contest"
	CategoryRating  Category = "rating"
	CategoryHack    Category = "hack"
	CategoryGroup   Category = "group"
	// CategorySystem covers everything that is not opt-out — account and
	// authoring events such as "your problem was approved". There is no
	// preference for these on purpose: a setter who submitted work for review
	// needs to learn the verdict.
	CategorySystem Category = "system"
)

// Notifier writes notifications. A nil *Notifier is valid and does nothing, so a
// caller that has no notifier wired (tests, the seeder) needs no nil checks.
type Notifier struct {
	store Store
}

func New(s Store) *Notifier {
	if s == nil {
		return nil
	}
	return &Notifier{store: s}
}

// Notify writes one notification to one user.
//
// Failures are logged and swallowed: a notification is a side effect of an
// action that has already succeeded (a submission was judged, a problem was
// approved), and failing that action because the courtesy copy failed would be
// worse than losing it.
func (n *Notifier) Notify(ctx context.Context, userID, typ, title, content, link string, cat Category) {
	if n == nil || n.store == nil || userID == "" {
		return
	}
	if !n.wants(ctx, userID, cat) {
		return
	}
	rec := &model.Notification{
		UserID:  userID,
		Type:    typ,
		Title:   title,
		Content: content,
		Link:    link,
	}
	if err := n.store.Create(ctx, rec); err != nil {
		slog.Error("notify: create failed", "user_id", userID, "type", typ, "error", err)
	}
}

// NotifyMany writes the same notification to several users, skipping the user who
// caused it — nobody needs telling about their own action.
func (n *Notifier) NotifyMany(ctx context.Context, userIDs []string, actorID, typ, title, content, link string, cat Category) {
	for _, uid := range userIDs {
		if uid == actorID {
			continue
		}
		n.Notify(ctx, uid, typ, title, content, link, cat)
	}
}

// wants reports whether the user's preferences allow this category. An
// unreadable preference row means "yes": the defaults are all on, and dropping a
// notification because of a lookup error is the behaviour we are fixing.
func (n *Notifier) wants(ctx context.Context, userID string, cat Category) bool {
	p, err := n.store.GetPreferences(ctx, userID)
	if err != nil || p == nil {
		return true
	}
	switch cat {
	case CategoryContest:
		return p.ContestAnnouncements
	case CategoryRating:
		return p.RatingChanges
	case CategoryHack:
		return p.HackNotifications
	case CategoryGroup:
		return p.GroupActivities
	default:
		// System notifications have no opt-out.
		return true
	}
}

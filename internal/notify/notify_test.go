package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The bell, the unread count and the preferences page all existed for a feature
 * that never wrote a row: NotificationStore.Create had no callers. These tests pin
 * the producer, including the part that was entirely missing before — actually
 * consulting the preferences the user had a UI for.
 */

type fakeStore struct {
	created []model.Notification
	prefs   *model.NotificationPreferences
	err     error
}

func (f *fakeStore) Create(_ context.Context, n *model.Notification) error {
	f.created = append(f.created, *n)
	return nil
}

func (f *fakeStore) GetPreferences(_ context.Context, userID string) (*model.NotificationPreferences, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.prefs != nil {
		return f.prefs, nil
	}
	// Mirror the store's default: everything on.
	return &model.NotificationPreferences{
		UserID: userID, ContestAnnouncements: true, RatingChanges: true,
		HackNotifications: true, GroupActivities: true,
	}, nil
}

func TestNotifyWritesARow(t *testing.T) {
	store := &fakeStore{}
	n := New(store)

	n.Notify(context.Background(), "u1", "problem_review", "approved", "Two Sum", "/problems/two-sum", CategorySystem)

	if len(store.created) != 1 {
		t.Fatalf("wrote %d notifications, want 1", len(store.created))
	}
	got := store.created[0]
	if got.UserID != "u1" || got.Type != "problem_review" || got.Link != "/problems/two-sum" {
		t.Fatalf("unexpected notification: %+v", got)
	}
}

// The preferences page existed but nothing read it.
func TestNotifyHonoursPreferences(t *testing.T) {
	cases := []struct {
		name  string
		prefs *model.NotificationPreferences
		cat   Category
		want  int
	}{
		{"contest on", &model.NotificationPreferences{ContestAnnouncements: true}, CategoryContest, 1},
		{"contest off", &model.NotificationPreferences{ContestAnnouncements: false}, CategoryContest, 0},
		{"rating off", &model.NotificationPreferences{RatingChanges: false}, CategoryRating, 0},
		{"hack off", &model.NotificationPreferences{HackNotifications: false}, CategoryHack, 0},
		{"group off", &model.NotificationPreferences{GroupActivities: false}, CategoryGroup, 0},
		// System events have no opt-out: a setter must learn a review verdict.
		{"system ignores prefs", &model.NotificationPreferences{}, CategorySystem, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeStore{prefs: tc.prefs}
			New(store).Notify(context.Background(), "u1", "t", "title", "body", "", tc.cat)
			if len(store.created) != tc.want {
				t.Fatalf("wrote %d, want %d", len(store.created), tc.want)
			}
		})
	}
}

// A preference lookup failure must not silently drop the notification: the
// defaults are all on, and dropping is the bug being fixed.
func TestNotifyProceedsWhenPreferencesFailToLoad(t *testing.T) {
	store := &fakeStore{err: errors.New("db down")}

	New(store).Notify(context.Background(), "u1", "t", "title", "body", "", CategoryContest)

	if len(store.created) != 1 {
		t.Fatalf("wrote %d, want 1 despite the preference lookup failing", len(store.created))
	}
}

func TestNotifyManySkipsTheActor(t *testing.T) {
	store := &fakeStore{}

	New(store).NotifyMany(context.Background(), []string{"u1", "u2"}, "u1", "t", "title", "body", "", CategorySystem)

	if len(store.created) != 1 {
		t.Fatalf("wrote %d, want 1 (the actor is skipped)", len(store.created))
	}
	if store.created[0].UserID != "u2" {
		t.Fatalf("notified %q, want u2", store.created[0].UserID)
	}
}

// A nil notifier is a valid no-op, which is what lets the handlers stay
// constructible without one in tests and in the seeder.
func TestNilNotifierIsANoOp(t *testing.T) {
	var n *Notifier
	n.Notify(context.Background(), "u1", "t", "title", "body", "", CategorySystem)
	n.NotifyMany(context.Background(), []string{"u1"}, "", "t", "title", "body", "", CategorySystem)
}

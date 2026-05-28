package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

type mockNotifStore struct {
	created     []*model.Notification
	prefs       map[string]*model.NotificationPreferences
	getPrefsErr error
	createErr   error
}

func (m *mockNotifStore) Create(_ context.Context, n *model.Notification) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, n)
	return nil
}

func (m *mockNotifStore) GetByUser(_ context.Context, _ string, _ bool, _ int) ([]model.Notification, error) {
	return nil, nil
}

func (m *mockNotifStore) MarkAsRead(_ context.Context, _ string) error   { return nil }
func (m *mockNotifStore) MarkAllAsRead(_ context.Context, _ string) error { return nil }
func (m *mockNotifStore) GetUnreadCount(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *mockNotifStore) Delete(_ context.Context, _ string) error        { return nil }

func (m *mockNotifStore) GetPreferences(_ context.Context, userID string) (*model.NotificationPreferences, error) {
	if m.getPrefsErr != nil {
		return nil, m.getPrefsErr
	}
	if p, ok := m.prefs[userID]; ok {
		return p, nil
	}
	return &model.NotificationPreferences{
		UserID:               userID,
		ContestAnnouncements: true,
		RatingChanges:        true,
		HackNotifications:    true,
		GroupActivities:      true,
	}, nil
}

func (m *mockNotifStore) UpdatePreferences(_ context.Context, _ string, _ *model.NotificationPreferences) error {
	return nil
}

func TestService_SendNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("successful send", func(t *testing.T) {
		mock := &mockNotifStore{}
		svc := NewService(mock, nil)

		err := svc.SendNotification(ctx, model.CreateNotificationRequest{
			UserID: "user-1", Type: "rating", Title: "Rating Update", Content: "Your rating changed",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mock.created) != 1 {
			t.Fatalf("expected 1 notification, got %d", len(mock.created))
		}
		if mock.created[0].Title != "Rating Update" {
			t.Errorf("title = %q, want %q", mock.created[0].Title, "Rating Update")
		}
	})

	t.Run("disabled preference skips send", func(t *testing.T) {
		mock := &mockNotifStore{
			prefs: map[string]*model.NotificationPreferences{
				"user-1": {UserID: "user-1", RatingChanges: false},
			},
		}
		svc := NewService(mock, nil)

		err := svc.SendNotification(ctx, model.CreateNotificationRequest{
			UserID: "user-1", Type: "rating", Title: "Rating Update", Content: "Your rating changed",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mock.created) != 0 {
			t.Errorf("expected 0 notifications (preference disabled), got %d", len(mock.created))
		}
	})

	t.Run("unknown type defaults to enabled", func(t *testing.T) {
		mock := &mockNotifStore{}
		svc := NewService(mock, nil)

		err := svc.SendNotification(ctx, model.CreateNotificationRequest{
			UserID: "user-1", Type: "unknown_type", Title: "Test", Content: "Content",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mock.created) != 1 {
			t.Errorf("expected 1 notification (unknown type = enabled), got %d", len(mock.created))
		}
	})

	t.Run("store error propagates", func(t *testing.T) {
		mock := &mockNotifStore{createErr: errors.New("db error")}
		svc := NewService(mock, nil)

		err := svc.SendNotification(ctx, model.CreateNotificationRequest{
			UserID: "user-1", Type: "rating", Title: "Test", Content: "Content",
		})
		if err == nil {
			t.Error("expected error from store")
		}
	})

	t.Run("get preferences error propagates", func(t *testing.T) {
		mock := &mockNotifStore{getPrefsErr: errors.New("prefs error")}
		svc := NewService(mock, nil)

		err := svc.SendNotification(ctx, model.CreateNotificationRequest{
			UserID: "user-1", Type: "rating", Title: "Test", Content: "Content",
		})
		if err == nil {
			t.Error("expected error from get preferences")
		}
	})
}

func TestService_isNotificationEnabled(t *testing.T) {
	svc := NewService(nil, nil)

	tests := []struct {
		name     string
		notifType string
		prefs    *model.NotificationPreferences
		expected bool
	}{
		{"contest enabled", "contest", &model.NotificationPreferences{ContestAnnouncements: true}, true},
		{"contest disabled", "contest", &model.NotificationPreferences{ContestAnnouncements: false}, false},
		{"rating enabled", "rating", &model.NotificationPreferences{RatingChanges: true}, true},
		{"rating disabled", "rating", &model.NotificationPreferences{RatingChanges: false}, false},
		{"hack enabled", "hack", &model.NotificationPreferences{HackNotifications: true}, true},
		{"hack disabled", "hack", &model.NotificationPreferences{HackNotifications: false}, false},
		{"group enabled", "group", &model.NotificationPreferences{GroupActivities: true}, true},
		{"group disabled", "group", &model.NotificationPreferences{GroupActivities: false}, false},
		{"unknown type defaults true", "unknown", &model.NotificationPreferences{}, true},
		{"system type defaults true", "system", &model.NotificationPreferences{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.isNotificationEnabled(tt.prefs, tt.notifType)
			if got != tt.expected {
				t.Errorf("isNotificationEnabled(%q) = %v, want %v",
					tt.notifType, got, tt.expected)
			}
		})
	}
}

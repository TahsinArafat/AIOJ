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

func (s *Service) SendNotification(ctx context.Context, req model.CreateNotificationRequest) error {
	prefs, err := s.notifStore.GetPreferences(ctx, req.UserID)
	if err != nil {
		return err
	}

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

package handler

import (
	"context"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// stubUserForEV is a minimal UserStore for email-verification tests.
type stubUserForEV struct {
	users      map[string]*model.User
	byEmail    map[string]*model.User
	marked     string
	isVerified bool
}

func (s *stubUserForEV) Create(_ context.Context, u *model.User) error {
	if s.users == nil {
		s.users = map[string]*model.User{}
	}
	s.users[u.ID] = u
	if u.Email != "" {
		if s.byEmail == nil {
			s.byEmail = map[string]*model.User{}
		}
		s.byEmail[u.Email] = u
	}
	return nil
}

func (s *stubUserForEV) GetByID(_ context.Context, id string) (*model.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (s *stubUserForEV) GetByUsername(_ context.Context, name string) (*model.User, error) {
	for _, u := range s.users {
		if u.Username == name {
			return u, nil
		}
	}
	return nil, nil
}

func (s *stubUserForEV) GetByEmail(_ context.Context, e string) (*model.User, error) {
	if s.byEmail != nil {
		if u, ok := s.byEmail[e]; ok {
			return u, nil
		}
	}
	for _, u := range s.users {
		if u.Email == e {
			return u, nil
		}
	}
	return nil, nil
}

func (s *stubUserForEV) GetPublicProfile(context.Context, string) (*model.PublicProfile, error) {
	return nil, nil
}
func (s *stubUserForEV) GetProfile(context.Context, string) (*model.UserProfile, error) {
	return nil, nil
}
func (s *stubUserForEV) UpdateProfile(context.Context, string, *model.UserProfile) error {
	return nil
}
func (s *stubUserForEV) ListUsers(context.Context, int, int) ([]model.User, int, error) {
	return nil, 0, nil
}
func (s *stubUserForEV) UpdateRole(context.Context, string, string) error { return nil }
func (s *stubUserForEV) UpdatePassword(context.Context, string, string) error {
	return nil
}
func (s *stubUserForEV) UpdateRating(context.Context, string, int, int, int) error {
	return nil
}
func (s *stubUserForEV) MarkEmailVerified(_ context.Context, id string) error {
	s.marked = id
	s.isVerified = true
	return nil
}
func (s *stubUserForEV) IsEmailVerified(context.Context, string) (bool, error) {
	return s.isVerified, nil
}

// stubEVT is a minimal EmailVerificationTokenStore for tests.
type stubEVT struct {
	tok *model.EmailVerificationToken
}

func (s *stubEVT) Create(context.Context, string, string, string, time.Time) error {
	return nil
}
func (s *stubEVT) GetByHash(context.Context, string) (*model.EmailVerificationToken, error) {
	return s.tok, nil
}
func (s *stubEVT) MarkUsed(context.Context, string) error { return nil }

// stubPasswordResetForMail is a minimal PasswordResetTokenStore for tests.
type stubPasswordResetForMail struct{}

func (s *stubPasswordResetForMail) Create(context.Context, string, string, string, time.Time) error {
	return nil
}
func (s *stubPasswordResetForMail) GetByHash(context.Context, string) (*model.PasswordResetToken, error) {
	return nil, nil
}
func (s *stubPasswordResetForMail) MarkUsed(context.Context, string) error { return nil }

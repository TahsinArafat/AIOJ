package handler

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Four training-plan mutations had no ownership check at all: any signed-in
 * account could add a section to, or delete a problem from, anyone's plan by id.
 * The section and problem routes carry no plan id, so those handlers must walk
 * section -> plan before authorizing. canManageTrainingPlan is that decision, and
 * it now also backs the two handlers (Update, Delete) that already had it inline.
 */

type orgRoleStub struct {
	// roles maps "orgID/userID" -> role. Missing means non-member.
	roles map[string]string
	err   error
}

func (s orgRoleStub) GetMemberRole(_ context.Context, orgID, userID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.roles[orgID+"/"+userID], nil
}

func planInOrg(orgID string) *model.TrainingPlan {
	return &model.TrainingPlan{ID: "plan1", OrganizationID: &orgID, CreatedBy: "creator1"}
}

func personalPlan() *model.TrainingPlan {
	return &model.TrainingPlan{ID: "plan1", CreatedBy: "creator1"}
}

func TestCanManageTrainingPlan(t *testing.T) {
	cases := []struct {
		name   string
		plan   *model.TrainingPlan
		claims *auth.Claims
		roles  map[string]string
		want   bool
	}{
		{"anonymous", personalPlan(), nil, nil, false},
		{"nil plan", nil, &auth.Claims{UserID: "creator1", Role: "setter"}, nil, false},

		// Personal plans: the creator, and admins.
		{"personal plan creator", personalPlan(), &auth.Claims{UserID: "creator1", Role: "setter"}, nil, true},
		{"personal plan stranger", personalPlan(), &auth.Claims{UserID: "other", Role: "setter"}, nil, false},
		{"personal plan admin", personalPlan(), &auth.Claims{UserID: "admin1", Role: "admin"}, nil, true},

		// Organization plans: the org's owner/admin, not merely a member.
		{"org owner", planInOrg("org1"), &auth.Claims{UserID: "u1", Role: "setter"}, map[string]string{"org1/u1": "owner"}, true},
		{"org admin", planInOrg("org1"), &auth.Claims{UserID: "u1", Role: "setter"}, map[string]string{"org1/u1": "admin"}, true},
		{"org plain member is refused", planInOrg("org1"), &auth.Claims{UserID: "u1", Role: "setter"}, map[string]string{"org1/u1": "member"}, false},
		{"org non-member", planInOrg("org1"), &auth.Claims{UserID: "u1", Role: "setter"}, nil, false},
		{"org creator without a role is refused", planInOrg("org1"), &auth.Claims{UserID: "creator1", Role: "setter"}, nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orgs := orgRoleStub{roles: tc.roles}
			if got := canManageTrainingPlan(context.Background(), orgs, tc.plan, tc.claims); got != tc.want {
				t.Fatalf("canManageTrainingPlan = %v, want %v", got, tc.want)
			}
		})
	}
}

// A store failure must deny, not allow: failing open here would grant write access
// to a plan during a database hiccup.
func TestCanManageTrainingPlanFailsClosedOnStoreError(t *testing.T) {
	orgs := orgRoleStub{err: context.DeadlineExceeded}

	got := canManageTrainingPlan(context.Background(), orgs, planInOrg("org1"),
		&auth.Claims{UserID: "u1", Role: "setter"})

	if got {
		t.Fatal("a store error granted plan management")
	}
}

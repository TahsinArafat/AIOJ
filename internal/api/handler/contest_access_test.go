package handler

import (
	"context"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * canReadContest is the single answer to "may this caller see this contest".
 * Four handlers had hand-copied it, and the three that needed a fifth copy
 * (gym, virtual, group) never wrote one — which is how a gym could be pointed at
 * a private draft and publish its title to the gym listing.
 */

type contestAccessStub struct{ access bool }

func (s contestAccessStub) HasAccess(context.Context, string, string, ...string) bool {
	return s.access
}

func startedContest() *model.Contest {
	return &model.Contest{
		ID: "c1", Visible: true,
		StartTime: time.Now().Add(-time.Hour), EndTime: time.Now().Add(time.Hour),
		CreatedBy: "creator1",
	}
}

func draftContest() *model.Contest {
	c := startedContest()
	c.Visible = false
	return c
}

func unstartedContest() *model.Contest {
	c := startedContest()
	c.StartTime = time.Now().Add(time.Hour)
	return c
}

func TestCanReadContest(t *testing.T) {
	cases := []struct {
		name    string
		contest *model.Contest
		claims  *auth.Claims
		access  bool
		want    bool
	}{
		{"published and started is public", startedContest(), nil, false, true},
		{"anonymous cannot read a draft", draftContest(), nil, false, false},
		{"anonymous cannot read an unstarted round", unstartedContest(), nil, false, false},
		{"nil contest", nil, &auth.Claims{UserID: "u", Role: "admin"}, false, false},

		{"plain user cannot read a draft", draftContest(), &auth.Claims{UserID: "u", Role: "user"}, false, false},
		{"admin can read a draft", draftContest(), &auth.Claims{UserID: "a", Role: "admin"}, false, true},
		{"creator can read a draft", draftContest(), &auth.Claims{UserID: "creator1", Role: "setter"}, false, true},
		{"manager can read a draft", draftContest(), &auth.Claims{UserID: "m", Role: "setter"}, true, true},
		{"judge can read an unstarted round", unstartedContest(), &auth.Claims{UserID: "j", Role: "setter"}, true, true},
		// The reported Gym bug in one line: a setter who is not staff gets nothing.
		{"unrelated setter cannot read a draft", draftContest(), &auth.Claims{UserID: "s2", Role: "setter"}, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canReadContest(context.Background(), contestAccessStub{access: tc.access}, tc.contest, tc.claims)
			if got != tc.want {
				t.Fatalf("canReadContest = %v, want %v", got, tc.want)
			}
		})
	}
}

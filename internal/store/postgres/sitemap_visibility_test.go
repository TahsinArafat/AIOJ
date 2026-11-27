package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/testdb"
)

/**
 * The sitemap and the share card must agree about what is public, because a
 * crawler that is told about a URL it cannot preview is worse than one told
 * nothing: the sitemap advertises a page that 404s.
 *
 * Each assertion below is paired with the public read that serves the same
 * entity, so a predicate that drifts between the two fails here rather than in
 * production. The blog pair is the sharpest: GetPostByID deliberately stays
 * unfiltered so an author can edit a draft, and GetPublicPostByID is the one the
 * public handler uses.
 */
func TestSitemapAndPublicReadsAgreeOnVisibility(t *testing.T) {
	db := testdb.Require(t)

	ctx := context.Background()
	userStore := NewUserStore(db)
	blogStore := NewBlogStore(db)
	groupStore := NewGroupStore(db)
	teamStore := NewTeamStore(db)
	sitemapStore := NewSitemapStore(db)
	contestStore := NewContestStore(db)
	gymStore := NewGymStore(db)
	trainingStore := NewTrainingPlanStore(db)
	orgStore := NewOrganizationStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "seo_" + uuid.New().String()[:8],
		Email:        "seo_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, "DELETE FROM blog_posts WHERE user_id = $1", owner.ID)
		db.ExecContext(ctx, "DELETE FROM groups WHERE created_by = $1", owner.ID)
		db.ExecContext(ctx, "DELETE FROM teams WHERE created_by = $1", owner.ID)
		db.ExecContext(ctx, "DELETE FROM training_plans WHERE created_by = $1", owner.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
	})

	origin := "https://example.test"
	contains := func(entries []SitemapEntry, id string) bool {
		for _, e := range entries {
			if e.Location == origin+"/blog/"+id || e.Location == origin+"/groups/"+id ||
				e.Location == origin+"/teams/"+id || e.Location == origin+"/training/"+id ||
				e.Location == origin+"/gym/"+id {
				return true
			}
		}
		return false
	}

	t.Run("blog post visibility", func(t *testing.T) {
		visible := &model.BlogPost{UserID: owner.ID, Title: "Visible", Content: "x", Tags: []string{}}
		if err := blogStore.CreatePost(ctx, visible); err != nil {
			t.Fatalf("create visible: %v", err)
		}
		hidden := &model.BlogPost{UserID: owner.ID, Title: "Hidden", Content: "x", Tags: []string{}}
		if err := blogStore.CreatePost(ctx, hidden); err != nil {
			t.Fatalf("create hidden: %v", err)
		}
		// visible = false is the column migration 000058 added; the other two
		// predicates are exercised by the same query.
		if _, err := db.ExecContext(ctx,
			"UPDATE blog_posts SET visible = false WHERE id = $1", hidden.ID); err != nil {
			t.Fatalf("hide post: %v", err)
		}

		got, err := blogStore.GetPublicPostByID(ctx, visible.ID)
		if err != nil || got == nil {
			t.Fatalf("public read of a visible post = (%v, %v), want the post", got, err)
		}
		// The unfiltered getter must still return the draft, or an author cannot
		// edit it.
		got, err = blogStore.GetPostByID(ctx, hidden.ID)
		if err != nil || got == nil {
			t.Fatalf("raw read of a hidden post = (%v, %v), want the post for editing", got, err)
		}
		got, err = blogStore.GetPublicPostByID(ctx, hidden.ID)
		if err != nil {
			t.Fatalf("public read: %v", err)
		}
		if got != nil {
			t.Fatalf("public read returned a hidden post — the visibility rule is not applied")
		}

		entries, err := sitemapStore.PublicBlogPosts(ctx, origin)
		if err != nil {
			t.Fatalf("PublicBlogPosts: %v", err)
		}
		if !contains(entries, visible.ID) {
			t.Errorf("sitemap omitted the visible post")
		}
		if contains(entries, hidden.ID) {
			t.Errorf("sitemap advertised the hidden post")
		}
	})

	t.Run("group and team is_public", func(t *testing.T) {
		pub := &model.Group{Name: "Public", IsPublic: true, CreatedBy: owner.ID, JoinPolicy: "auto_approve"}
		if err := groupStore.Create(ctx, pub); err != nil {
			t.Fatalf("create public group: %v", err)
		}
		priv := &model.Group{Name: "Private", IsPublic: false, CreatedBy: owner.ID, JoinPolicy: "manual_approve"}
		if err := groupStore.Create(ctx, priv); err != nil {
			t.Fatalf("create private group: %v", err)
		}
		// GetByID returns is_public rather than filtering on it; the handler and
		// the card closure are what gate.
		g, err := groupStore.GetByID(ctx, priv.ID)
		if err != nil || g == nil {
			t.Fatalf("GetByID private group = (%v, %v)", g, err)
		}
		if g.IsPublic {
			t.Fatalf("private group came back with is_public = true")
		}

		pubTeam := &model.Team{Name: "Public Team", IsPublic: true, CreatedBy: owner.ID}
		if err := teamStore.Create(ctx, pubTeam); err != nil {
			t.Fatalf("create public team: %v", err)
		}
		privTeam := &model.Team{Name: "Private Team", IsPublic: false, CreatedBy: owner.ID}
		if err := teamStore.Create(ctx, privTeam); err != nil {
			t.Fatalf("create private team: %v", err)
		}

		groups, err := sitemapStore.PublicGroups(ctx, origin)
		if err != nil {
			t.Fatalf("PublicGroups: %v", err)
		}
		if !contains(groups, pub.ID) {
			t.Errorf("sitemap omitted the public group")
		}
		if contains(groups, priv.ID) {
			t.Errorf("sitemap advertised the private group")
		}

		teams, err := sitemapStore.PublicTeams(ctx, origin)
		if err != nil {
			t.Fatalf("PublicTeams: %v", err)
		}
		if !contains(teams, pubTeam.ID) {
			t.Errorf("sitemap omitted the public team")
		}
		if contains(teams, privTeam.ID) {
			t.Errorf("sitemap advertised the private team")
		}
	})

	t.Run("training plan public is derived from a null organization", func(t *testing.T) {
		pub := &model.TrainingPlan{Title: "Public Plan", CreatedBy: owner.ID}
		if err := trainingStore.Create(ctx, pub); err != nil {
			t.Fatalf("create public plan: %v", err)
		}
		org := &model.Organization{Name: "Org " + uuid.New().String()[:8], CreatedBy: owner.ID}
		if err := orgStore.Create(ctx, org); err != nil {
			t.Fatalf("create org: %v", err)
		}
		orgID := org.ID
		priv := &model.TrainingPlan{Title: "Org Plan", CreatedBy: owner.ID, OrganizationID: &orgID}
		if err := trainingStore.Create(ctx, priv); err != nil {
			t.Fatalf("create org plan: %v", err)
		}

		got, err := trainingStore.GetByID(ctx, pub.ID)
		if err != nil || got == nil || !got.IsPublic {
			t.Fatalf("org-less plan IsPublic = %v (err %v), want true", got, err)
		}
		got, err = trainingStore.GetByID(ctx, priv.ID)
		if err != nil || got == nil || got.IsPublic {
			t.Fatalf("org-owned plan IsPublic = %v (err %v), want false", got, err)
		}

		entries, err := sitemapStore.PublicTrainingPlans(ctx, origin)
		if err != nil {
			t.Fatalf("PublicTrainingPlans: %v", err)
		}
		if !contains(entries, pub.ID) {
			t.Errorf("sitemap omitted the public plan")
		}
		if contains(entries, priv.ID) {
			t.Errorf("sitemap advertised the org-owned plan")
		}
	})

	t.Run("gym needs a published contest", func(t *testing.T) {
		start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		pubContest := &model.Contest{
			ID: uuid.New().String(), Title: "Published", Type: "acm", Format: "acm",
			StartTime: start, EndTime: start.Add(2 * time.Hour),
			CreatedBy: owner.ID, Status: model.ContestStatusPublished, Visible: true,
		}
		if err := contestStore.Create(ctx, pubContest); err != nil {
			t.Fatalf("create published contest: %v", err)
		}
		draftContest := &model.Contest{
			ID: uuid.New().String(), Title: "Draft", Type: "acm", Format: "acm",
			StartTime: start, EndTime: start.Add(2 * time.Hour),
			CreatedBy: owner.ID, Status: model.ContestStatusDraft, Visible: false,
		}
		if err := contestStore.Create(ctx, draftContest); err != nil {
			t.Fatalf("create draft contest: %v", err)
		}
		t.Cleanup(func() {
			db.ExecContext(ctx, "DELETE FROM gym_contests WHERE created_by = $1", owner.ID)
			db.ExecContext(ctx, "DELETE FROM contests WHERE created_by = $1", owner.ID)
		})

		pubGym := &model.GymContest{ContestID: pubContest.ID, ContestTitle: "Published", IsPublic: true, CreatedBy: owner.ID}
		if err := gymStore.Create(ctx, pubGym); err != nil {
			t.Fatalf("create public gym: %v", err)
		}
		draftGym := &model.GymContest{ContestID: draftContest.ID, ContestTitle: "Draft", IsPublic: true, CreatedBy: owner.ID}
		if err := gymStore.Create(ctx, draftGym); err != nil {
			t.Fatalf("create draft gym: %v", err)
		}

		// The anonymous read already refuses a gym on an unpublished contest.
		got, err := gymStore.GetByID(ctx, draftGym.ID)
		if err != nil {
			t.Fatalf("gym read: %v", err)
		}
		if got != nil {
			t.Fatalf("anonymous read returned a gym on an unpublished contest")
		}

		entries, err := sitemapStore.PublicGyms(ctx, origin)
		if err != nil {
			t.Fatalf("PublicGyms: %v", err)
		}
		if !contains(entries, pubGym.ID) {
			t.Errorf("sitemap omitted the public gym")
		}
		if contains(entries, draftGym.ID) {
			t.Errorf("sitemap advertised a gym on an unpublished contest")
		}
	})
}

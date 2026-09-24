-- UserStore.Create only ever inserted into `users`; the matching `user_profiles`
-- row was created lazily by GetProfile the first time somebody opened a profile.
-- Users who had never had their profile opened therefore had no profile row, and
-- ListUsersByRating's INNER JOIN silently dropped them. On a seeded database that
-- left the leaderboard and the home page's "Top Rated Users" widget permanently
-- empty while /api/stats still reported every user.

-- Backfill every user that is missing a profile. All NOT NULL columns have
-- defaults, so the defaults are the correct starting state for a new user.
INSERT INTO user_profiles (user_id)
SELECT u.id
FROM users u
LEFT JOIN user_profiles up ON up.user_id = u.id
WHERE up.user_id IS NULL
ON CONFLICT (user_id) DO NOTHING;

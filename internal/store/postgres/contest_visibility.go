package postgres

// contestVisibilityPredicate is the one definition of "this round may be listed".
//
// Three separate queries publish contests into public surfaces — the catalog
// (ListWithDivision), global search (SearchContests) and the sitemap
// (PublicContests) — and each had grown its own, wrong, idea of what "public"
// means. The sitemap's own comment claimed it returned "non-private contests"
// while filtering only on visible, so password-protected rounds were being handed
// to search engines.
//
// Sharing one predicate string is deliberate: the drift is what broke this, so the
// rule lives in one place and every caller formats it with its own placeholder.
//
// Args a caller must supply, in order:
//
//	$n   viewer UUID as text ('' for an anonymous caller)
//
// A round is listable when it has no access restriction, or when the viewer is
// allowed to see it anyway. Staff (creator, manager/judge/tester) always are —
// they need to find their own drafts in the list.
const contestVisibilityPredicate = `(
	-- no password, and no group restriction: an unremarkable public round
	(password_hash = ''
	 AND NOT EXISTS (SELECT 1 FROM group_contests gc WHERE gc.contest_id = contests.id)) 
	-- or: the viewer appeared earlier as $%d and is entitled to see it
	OR ($%d <> '' AND (
		created_by::text = $%d
		OR EXISTS (SELECT 1 FROM contest_permissions cp
		            WHERE cp.contest_id = contests.id AND cp.user_id::text = $%d
		              AND cp.access_level IN ('manager','judge','tester'))
		OR (password_hash <> ''
		    AND EXISTS (SELECT 1 FROM contest_access_grants g
		                 WHERE g.contest_id = contests.id AND g.user_id::text = $%d))
		OR EXISTS (SELECT 1 FROM group_members gm
		            JOIN group_contests gc ON gm.group_id = gc.group_id
		            WHERE gc.contest_id = contests.id AND gm.user_id::text = $%d)
	))
)`

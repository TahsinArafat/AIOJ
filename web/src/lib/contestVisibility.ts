/**
 * The access half of a contest's settings payload.
 *
 * Two gates live here, and neither of them is catalog presence:
 *   - password: who may OPEN a private round.
 *   - group_id: which group may open a restricted one.
 *
 * `visible` deliberately does not appear. It used to be sent as
 * `visibility !== 'private'`, and the server now discards it outright
 * (handler/contest.go Update, "_ = req.Visible") because a round is visible when
 * its status is published — the store derives the column from status. Whether a
 * round is advertised is the separate `listed` field. Sending `visible` made this
 * form look like it controlled publication when it controlled nothing.
 *
 * Lives in lib/ rather than next to the page so the rule can be unit-tested
 * without pulling a component into the test's module graph (react-refresh).
 */
export function contestVisibilityPayload(form: {
    visibility: string
    password: string
    group_id: string
}) {
    return {
        // undefined = leave the stored password alone; '' = remove it.
        password: form.visibility === 'private' ? (form.password.trim() || undefined) : '',
        group_id: form.visibility === 'group' ? form.group_id : '',
    }
}

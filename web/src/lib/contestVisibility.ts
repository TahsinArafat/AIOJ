/**
 * The visibility half of a contest's settings payload.
 *
 * The bug this prevents: the settings form offered "Private (Password/Invite
 * required)" but the save sent `visible: true` unconditionally. Choosing private
 * therefore produced a contest that was visible to everyone — and, before the
 * server-side fix, one whose password was never enforced either. The two settings
 * have to move together: a private round is hidden from the catalog and reached by
 * its direct link.
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
        visible: form.visibility !== 'private',
        // undefined = leave the stored password alone; '' = remove it.
        password: form.visibility === 'private' ? (form.password.trim() || undefined) : '',
        group_id: form.visibility === 'group' ? form.group_id : '',
    }
}

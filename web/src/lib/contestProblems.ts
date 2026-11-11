/**
 * A contest problem's letter.
 *
 * Mirrors `model.ContestProblemIndex` on the server, which is the authority: the
 * letter is a function of a problem's position in the round, not a stored fact.
 *
 * This existed in four places and each computed it differently — the problems
 * tab and standings cells read the stored `index`, while the standings header
 * and the PDF booklet each did `String.fromCharCode(65 + i)` over whatever list
 * they happened to hold. Once a problem was removed those lists disagreed, so
 * the same problem could be "B" on one tab and "A" in the standings. Server data
 * carries a correct `index` now; this is only for rendering a position with no
 * problem attached to it, and it matches the server's rule.
 */

/** The letters a contest can use; `contest_problems.index` is CHAR(1). */
export const MAX_CONTEST_PROBLEMS = 26

export function contestProblemIndex(position: number): string {
    const p = Math.min(Math.max(0, Math.trunc(position)), MAX_CONTEST_PROBLEMS - 1)
    return String.fromCharCode(65 + p)
}

/**
 * The letter to print for a problem.
 *
 * Prefers the server's stored `index` and falls back to the position only when
 * it is missing — a problem that arrived without one should still render
 * something sensible rather than an empty cell.
 */
export function problemLetter(
    problem: { index?: string | null } | undefined,
    position: number,
): string {
    return problem?.index || contestProblemIndex(position)
}

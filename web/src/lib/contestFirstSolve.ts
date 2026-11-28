/**
 * Who solved a problem first.
 *
 * This is a question about the clock, and it was answered with a penalty.
 *
 * The standings highlight used to compare `cell.time`, which is the problem's
 * contribution to the penalty total — solve minutes PLUS the wrong-attempt
 * penalty. On that number a clean solve two minutes in (2) beats a solve one
 * minute in that took a wrong attempt (1 + 20 = 21), so the board highlighted the
 * later solve. `time` answers "whose result cost less"; only the instant of the
 * accepted submission answers "who got there first", and the API carries it as
 * `first_ac_at`.
 */

export interface SolvableCell {
    solved: boolean
    /** Total attempts including the accepted one. Carried because the wire cell
     * has it, but deliberately NOT read here: the count says how costly a solve
     * was, never when it happened. */
    attempts?: number
    /** Penalty minutes for this problem, not the solve clock. */
    time?: number
    /** Instant of the accepted submission, when the format produces one. */
    first_ac_at?: string | null
}

export interface SolvableEntry {
    user_id: string
    problems: Record<string, SolvableCell | undefined>
}

/** A solved cell reduced to what ordering needs. */
interface SolveOrder {
    /** Accepted-submission instant in ms, or null when the cell carries none. */
    at: number | null
    /** Penalty minutes — the fallback for a cell with no instant to read. */
    penalty: number
    userId: string
}

/**
 * Whether one solve of a problem precedes another.
 *
 * Timestamps decide it whenever both cells carry one, which is every cell an ACM,
 * Codeforces or AtCoder round produces. Only a cell with no accepting verdict to
 * timestamp (an ioi/oi score cell) falls back to the penalty, and a timestamped
 * cell then simply wins: penalty minutes carry wrong-attempt time and a wall clock
 * does not, so the two are not commensurable and ranking one against the other is
 * the bug this replaces.
 */
function isEarlierSolve(a: SolveOrder, b: SolveOrder): boolean {
    if (a.at !== null && b.at !== null) return a.at < b.at
    if (a.at !== null) return true
    if (b.at !== null) return false
    return a.penalty < b.penalty
}

function solveOrderOf(userId: string, cell: SolvableCell): SolveOrder | null {
    if (!cell.solved) return null
    const at = cell.first_ac_at ? Date.parse(cell.first_ac_at) : Number.NaN
    return {
        at: Number.isNaN(at) ? null : at,
        penalty: cell.time ?? 0,
        userId,
    }
}

/**
 * The `user_id::letter` keys of the participants who solved each problem first.
 *
 * Only the entries it is given are considered, so a paginated board answers
 * "first on this page" rather than "first in the round" — the caller must pass
 * every entry it wants the answer to be about.
 */
export function firstSolveKeys(entries: readonly SolvableEntry[]): Set<string> {
    const earliest = new Map<string, SolveOrder>()
    for (const entry of entries) {
        for (const [letter, cell] of Object.entries(entry.problems ?? {})) {
            const order = cell ? solveOrderOf(entry.user_id, cell) : null
            if (!order) continue
            const best = earliest.get(letter)
            if (!best || isEarlierSolve(order, best)) earliest.set(letter, order)
        }
    }
    return new Set([...earliest].map(([letter, s]) => `${s.userId}::${letter}`))
}

import { describe, expect, it } from 'vitest'
import { firstSolveKeys, type SolvableCell } from './contestFirstSolve'

/**
 * First solver on the standings.
 *
 * The board used to award this to the lowest `cell.time`, which is the problem's
 * penalty — solve minutes plus 20 per wrong attempt. A clean solve two minutes in
 * therefore beat a solve one minute in that took a wrong attempt, and the board
 * highlighted the later solve. Reported from a live round; pinned here.
 */

const start = Date.parse('2026-03-01T10:00:00Z')

/** An accepted cell `minutes` into the round, with `wrong` wrong attempts before it. */
function solved(minutes: number, wrong = 0): SolvableCell {
    return {
        solved: true,
        attempts: wrong + 1,
        time: minutes + wrong * 20,
        first_ac_at: new Date(start + minutes * 60_000).toISOString(),
    }
}

function entry(user_id: string, problems: Record<string, SolvableCell>) {
    return { user_id, problems }
}

describe('firstSolveKeys', () => {
    it('awards the problem to the earlier solve, not the cheaper penalty', () => {
        // a: solved A at minute 1 with one wrong attempt → penalty 21.
        // b: solved A clean at minute 2 → penalty 2.
        const keys = firstSolveKeys([
            entry('a', { A: solved(1, 1) }),
            entry('b', { A: solved(2) }),
        ])
        expect(keys.has('a::A')).toBe(true)
        expect(keys.has('b::A')).toBe(false)
    })

    it('still picks the earliest when the wrong attempt came first', () => {
        // The penalty is what made the old rule wrong; the instant is unaffected
        // by how many attempts preceded it.
        const keys = firstSolveKeys([
            entry('a', { A: solved(30, 3) }),
            entry('b', { A: solved(31, 0) }),
        ])
        expect([...keys]).toEqual(['a::A'])
    })

    it('gives each problem to its own first solver', () => {
        const keys = firstSolveKeys([
            entry('a', { A: solved(1), B: solved(40) }),
            entry('b', { A: solved(2), B: solved(10) }),
        ])
        expect([...keys].sort()).toEqual(['a::A', 'b::B'])
    })

    it('ignores unsolved cells and wrong attempts', () => {
        const keys = firstSolveKeys([
            entry('a', { A: { solved: false, attempts: 3, time: 0 } }),
            entry('b', { A: solved(5) }),
        ])
        expect([...keys]).toEqual(['b::A'])
    })

    it('falls back to the penalty for a cell with no accepted verdict', () => {
        // ioi/oi pay out a score without a single AC, so there is no instant to
        // read and the penalty is the only ordering left.
        const keys = firstSolveKeys([
            entry('a', { A: { solved: true, attempts: 2, time: 90 } }),
            entry('b', { A: { solved: true, attempts: 1, time: 60 } }),
        ])
        expect([...keys]).toEqual(['b::A'])
    })

    it('prefers a timestamped solve over one it cannot date', () => {
        const keys = firstSolveKeys([
            entry('a', { A: { solved: true, attempts: 1, time: 1 } }),
            entry('b', { A: solved(120) }),
        ])
        expect([...keys]).toEqual(['b::A'])
    })

    it('breaks a tie in favour of the entry it saw first', () => {
        const keys = firstSolveKeys([
            entry('a', { A: solved(1) }),
            entry('b', { A: solved(1) }),
        ])
        expect([...keys]).toEqual(['a::A'])
    })

    it('reports nothing for an empty board', () => {
        expect(firstSolveKeys([]).size).toBe(0)
    })
})

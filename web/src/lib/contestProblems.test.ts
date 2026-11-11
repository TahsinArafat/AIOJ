import { describe, expect, it } from 'vitest'
import { contestProblemIndex, problemLetter, MAX_CONTEST_PROBLEMS } from './contestProblems'

/**
 * The contest problem letter.
 *
 * This was computed in four places with two different rules. The problems tab
 * and the standings cells read the stored `index`; the standings header and the
 * PDF booklet each did `'A' + position` over whatever list they held. Once a
 * problem was removed those lists disagreed, so the same problem showed as "B"
 * on one tab and "A" in the standings.
 *
 * `problemLetter` is now the only rendering rule, and it prefers the server's
 * value — position is a fallback for a row that arrived without one.
 */

describe('contestProblemIndex', () => {
    it('maps position to letter', () => {
        expect(contestProblemIndex(0)).toBe('A')
        expect(contestProblemIndex(1)).toBe('B')
        expect(contestProblemIndex(25)).toBe('Z')
    })

    it('clamps to the 26 letters the schema can store', () => {
        // contest_problems.index is CHAR(1); a 27th problem has no letter.
        expect(contestProblemIndex(26)).toBe('Z')
        expect(contestProblemIndex(999)).toBe('Z')
        expect(MAX_CONTEST_PROBLEMS).toBe(26)
    })

    it('clamps negative positions to A', () => {
        expect(contestProblemIndex(-1)).toBe('A')
    })
})

describe('problemLetter', () => {
    it("prefers the server's stored letter over the position", () => {
        // This is the bug: position 0 in the returned list, but the round says
        // this problem is "B" because one before it was removed.
        expect(problemLetter({ index: 'B' }, 0)).toBe('B')
        expect(problemLetter({ index: 'C' }, 1)).toBe('C')
    })

    it('falls back to the position when the row has no letter', () => {
        expect(problemLetter(undefined, 0)).toBe('A')
        expect(problemLetter({}, 2)).toBe('C')
        expect(problemLetter({ index: null }, 1)).toBe('B')
        expect(problemLetter({ index: '' }, 1)).toBe('B')
    })

    it('agrees with the server rule for a gapless round', () => {
        // After renumbering, position and stored letter coincide — which is the
        // invariant the fix restores.
        const renumbered = [{ index: 'A' }, { index: 'B' }, { index: 'C' }]
        renumbered.forEach((p, i) => {
            expect(problemLetter(p, i)).toBe(contestProblemIndex(i))
        })
    })
})
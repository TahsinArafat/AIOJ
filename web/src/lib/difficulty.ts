import type { BadgeTone } from '../components/ui/Badge'

/**
 * Difficulty vocabulary shared by every surface that shows it. It lived as a
 * copy-paste in ProblemDetail and ProblemList, which is how the two ended up
 * mapping the same value to different tones.
 */
export const DIFFICULTY_TONE: Record<string, BadgeTone> = {
    easy: 'ok',
    medium: 'warn',
    hard: 'bad',
}

import type { AIBrief } from '../types/api'

/**
 * Brief helpers and defaults for the AI authoring forms.
 *
 * Kept out of the component file so it exports components only (the
 * react-refresh/only-export-components lint rule).
 */

export const DEFAULT_BRIEF: AIBrief = {
    tags: ['dp'],
    rating: 1400,
    rating_min: 1300,
    rating_max: 1500,
    problem_type: 'standard',
    testcase_count: 10,
    has_subtasks: false,
    solver_mode: 'solo',
    event_context: 'practice',
    format_profile: 'general',
    expected_solve_minutes: 45,
    audience_level: 'intermediate',
    insight_emphasis: 'balanced',
    story_tone: 'neutral',
    secondary_topics: [],
    required_techniques: [],
    forbidden_techniques: [],
    constraint_guidance: '',
    additional_prompt: '',
}

/**
 * Move the target rating and keep the acceptable range consistent.
 *
 * The backend rejects a brief whose range does not contain the rating
 * ("rating range must contain rating and stay between 800 and 3500"), so the
 * range is widened here rather than failing at submit time.
 */
export function withRating(brief: AIBrief, rating: number): AIBrief {
    const min = Math.max(800, Math.min(brief.rating_min ?? rating - 100, rating))
    const max = Math.min(3500, Math.max(brief.rating_max ?? rating + 100, rating))
    return { ...brief, rating, rating_min: min, rating_max: max }
}

/** Clamp a range edit so it still contains the target rating and stays valid. */
export function withRange(brief: AIBrief, edge: 'min' | 'max', value: number): AIBrief {
    if (edge === 'min') {
        const min = Math.max(800, Math.min(value, brief.rating))
        return { ...brief, rating_min: min, rating_max: Math.max(brief.rating, brief.rating_max ?? brief.rating + 100) }
    }
    const max = Math.min(3500, Math.max(value, brief.rating))
    return { ...brief, rating_max: max, rating_min: Math.min(brief.rating, brief.rating_min ?? brief.rating - 100) }
}

/** True when the brief satisfies the backend's cross-field rules. */
export function briefIsValid(brief: AIBrief): boolean {
    if (brief.tags.length === 0) return false
    if (brief.rating < 800 || brief.rating > 3500) return false
    const min = brief.rating_min ?? brief.rating - 100
    const max = brief.rating_max ?? brief.rating + 100
    if (min < 800 || max > 3500 || min > brief.rating || max < brief.rating) return false
    if (brief.format_profile === 'icpc' && brief.has_subtasks) return false
    return true
}

export function ratingColor(r: number): string {
    if (r < 1200) return 'text-muted'
    if (r < 1400) return 'text-ok'
    if (r < 1600) return 'text-accent'
    if (r < 1900) return 'text-brand'
    if (r < 2100) return 'text-accent'
    if (r < 2300) return 'text-warn'
    if (r < 2400) return 'text-warn'
    return 'text-bad'
}

export function ratingLabel(r: number): string {
    if (r < 1200) return 'Newbie'
    if (r < 1400) return 'Pupil'
    if (r < 1600) return 'Specialist'
    if (r < 1900) return 'Expert'
    if (r < 2100) return 'Candidate Master'
    if (r < 2300) return 'Master'
    if (r < 2400) return 'International Master'
    if (r < 2600) return 'Grandmaster'
    if (r < 3000) return 'International Grandmaster'
    return 'Legendary Grandmaster'
}

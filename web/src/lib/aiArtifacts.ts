import type { AIArtifactSpec, AIBrief, AIBriefField } from '../types/api'

/**
 * The brief fields the server accepts for an artifact, and the answering of
 * "does this artifact use this field".
 *
 * The catalog comes from GET /api/ai/artifacts rather than being restated here,
 * because the same table drives the server's validation and the prompt it builds
 * (`internal/model/ai_artifact.go`). A copy in the frontend would be a second
 * list of scopes that can disagree with the one that decides — which is how the
 * form came to offer a topic and rating to an editorial job that ignores both.
 *
 * Pure logic, no JSX and no hooks, so the react-refresh boundary in web/src/lib
 * stays honest: the fetch that supplies the catalog is in hooks/useArtifacts.
 */
/**
 * One artifact's accepted fields, with the questions the form asks of it.
 *
 * An unknown artifact answers true to everything and false to "required":
 * rendering a control the server would ignore is a cosmetic bug, while hiding
 * one it expects would silently drop the operator's input.
 */
export class ArtifactScope {
    readonly spec: AIArtifactSpec | undefined

    constructor(spec: AIArtifactSpec | undefined) {
        this.spec = spec
    }

    /** Whether this artifact uses a field. */
    accepts(field: AIBriefField): boolean {
        if (!this.spec) return true
        return this.spec.fields.includes(field)
    }

    /**
     * Whether this artifact needs a field before a run can start.
     *
     * False when the catalog has not loaded. Requiring nothing is the safe
     * direction on an unknown scope: blocking the button would strand a setter on
     * a scope that never needed the field, while the server still rejects a brief
     * that is genuinely invalid and says which field is missing.
     */
    requires(field: AIBriefField): boolean {
        if (!this.spec) return false
        return field === 'topics' && this.accepts(field)
    }

    /** Whether any field in a group is used, so a section can be composed. */
    acceptsAny(...fields: AIBriefField[]): boolean {
        return fields.some(f => this.accepts(f))
    }
}

/**
 * The artifact specs that may be chosen for an existing problem.
 *
 * The whole-package scope is excluded: "improve this problem" does not mean
 * "regenerate everything it already has".
 */
export function offeredForExisting(specs: AIArtifactSpec[]): AIArtifactSpec[] {
    return specs.filter(s => s.offered_for_existing)
}

/**
 * How each brief property maps to the field the catalog names.
 *
 * The catalog names concepts; the wire uses JSON keys. They differ where the key
 * is abbreviated (`tags` for topics, `testcase_count` for the test-set size), so
 * the mapping is explicit rather than a string comparison that would silently
 * stop matching. A property absent from this table is kept, which is the safe
 * direction: a new brief field is sent until it is deliberately scoped.
 */
const BRIEF_KEY_FIELDS: Partial<Record<keyof AIBrief, AIBriefField>> = {
    tags: 'topics',
    rating: 'rating',
    rating_min: 'rating_range',
    rating_max: 'rating_range',
    format_profile: 'format_profile',
    solver_mode: 'solver_mode',
    team_size: 'solver_mode',
    event_context: 'event_context',
    insight_emphasis: 'insight_emphasis',
    expected_solve_minutes: 'expected_solve_minutes',
    primary_topics: 'primary_topics',
    secondary_topics: 'secondary_topics',
    required_techniques: 'required_techniques',
    forbidden_techniques: 'forbidden_techniques',
    constraint_guidance: 'constraint_guidance',
    audience_level: 'audience_level',
    story_tone: 'story_tone',
    testcase_count: 'testcase_count',
    has_subtasks: 'has_subtasks',
    additional_prompt: 'additional_prompt',
}

/**
 * Strip a brief down to the fields an artifact accepts.
 *
 * The form no longer edits fields the artifact ignores, but the brief it holds
 * keeps whatever the setter entered for a previous scope — so a run started
 * after switching artifacts must not carry a stale rating or test count into a
 * scope that has no use for it. The server would ignore those fields anyway; the
 * reason to drop them here is the job record, which renders the brief it was
 * given, and a stored editorial brief claiming "rating 1400" is a lie about what
 * the setter asked for.
 */
export function pruneBrief(brief: AIBrief, scope: ArtifactScope): AIBrief {
    // Nothing to prune against: keep the brief rather than empty it.
    if (!scope.spec) return brief
    const out: AIBrief = { ...brief }
    for (const key of Object.keys(brief) as Array<keyof AIBrief>) {
        const f = BRIEF_KEY_FIELDS[key]
        if (f && !scope.accepts(f)) {
            delete out[key]
        }
    }
    return out
}

/**
 * Per-format settings, as the server's format registry defines them.
 *
 * These were reachable from exactly one place: the create-contest page, which had a
 * hand-written block per format. That page is now name-only, so without this the
 * tuning would have become unreachable — a round could switch format in the
 * workspace but never set the format's own numbers, and the server's default would
 * silently stand.
 *
 * A declarative table rather than a component per format, because the five shapes
 * are the same two controls (a number and a checkbox) with different keys, and five
 * hand-written blocks is how one of them ends up sending a key the server does not
 * read. The keys here are the JSON tags from each format package's Config struct;
 * the test asserts they match what the server actually accepts, so a rename on
 * either side fails rather than silently storing a key nothing reads.
 *
 * Pure data + pure functions: no JSX, so this lives in lib/ and the settings tab
 * renders from it.
 */

export type FormatFieldType = 'number' | 'checkbox'

export interface FormatField {
    /** The JSON key the server's Config struct reads. */
    key: string
    label: string
    type: FormatFieldType
    /** Numbers only. */
    min?: number
    max?: number
    step?: number
    /** Shown under the control. */
    hint?: string
}

export interface FormatSpec {
    id: string
    label: string
    /** One line on what the format does to the board. */
    description: string
    fields: FormatField[]
}

/**
 * The formats, in the order the create page listed them.
 *
 * `full_problem`-style ids are the registry names (acm, oi, ioi, atcoder,
 * codeforces) — the same strings the server's format.Register uses, so a mismatch
 * is a round whose config the server rejects.
 */
export const FORMAT_SPECS: FormatSpec[] = [
    {
        id: 'acm',
        label: 'ACM/ICPC',
        description: 'Penalty-based: wrong submissions add time, ranked by problems solved then penalty.',
        fields: [
            { key: 'penalty_per_wrong', label: 'Penalty per wrong answer (minutes)', type: 'number', min: 0, step: 1 },
            { key: 'time_penalty', label: 'Apply a time penalty', type: 'checkbox' },
        ],
    },
    {
        id: 'oi',
        label: 'OI',
        description: 'Max score per problem, no time penalty.',
        fields: [
            { key: 'max_score_per_problem', label: 'Max score per problem', type: 'number', min: 1, step: 1 },
        ],
    },
    {
        id: 'ioi',
        label: 'IOI',
        description: 'Subtask-based scoring with partial credit.',
        fields: [
            { key: 'partial_credit', label: 'Award partial credit', type: 'checkbox' },
            { key: 'subtask_scoring', label: 'Score by subtask', type: 'checkbox' },
        ],
    },
    {
        id: 'atcoder',
        label: 'AtCoder',
        description: 'Penalty is the time of the accepted run; no wrong-attempt penalty.',
        fields: [
            { key: 'penalty_is_time_of_ac', label: 'Penalty is the accepted time', type: 'checkbox' },
            { key: 'no_wrong_attempt_penalty', label: 'Ignore wrong attempts', type: 'checkbox' },
        ],
    },
    {
        id: 'codeforces',
        label: 'Codeforces',
        description: 'Scores decay over time from a per-problem starting value.',
        fields: [
            {
                key: 'initial_scores',
                label: 'Initial scores',
                type: 'number',
                min: 0,
                step: 100,
                hint: 'One starting value per problem, in problem order. The create page derived ' +
                    'these from the problem count; editing them here is a comma-separated list.',
            },
            { key: 'decay_factor', label: 'Decay factor', type: 'number', min: 1, step: 1 },
            { key: 'min_score_ratio', label: 'Minimum score ratio', type: 'number', min: 0, max: 1, step: 0.05 },
            { key: 'wrong_submission_penalty', label: 'Wrong submission penalty', type: 'number', min: 0, step: 1 },
        ],
    },
]

export function formatSpec(id: string): FormatSpec | undefined {
    return FORMAT_SPECS.find((f) => f.id === id)
}

/** The default config for a format, as the server would store it. */
export function defaultFormatConfig(id: string): Record<string, unknown> {
    switch (id) {
        case 'acm':
            return { penalty_per_wrong: 20, time_penalty: true }
        case 'oi':
            return { max_score_per_problem: 100 }
        case 'ioi':
            return { partial_credit: true, subtask_scoring: true }
        case 'atcoder':
            return { penalty_is_time_of_ac: true, no_wrong_attempt_penalty: true }
        case 'codeforces':
            return {
                initial_scores: [500, 1000, 1500, 2000, 2500],
                decay_factor: 250,
                min_score_ratio: 0.3,
                wrong_submission_penalty: 50,
            }
        default:
            return {}
    }
}

/**
 * A stored config rendered as editable strings.
 *
 * Strings because the inputs are text: a number input bound to a number cannot
 * represent an empty box, and an empty box is how a setter clears a field to type a
 * new value. The conversion back happens in configFromForm.
 */
export function formFromFormatConfig(id: string, raw: unknown): Record<string, string> {
    const spec = formatSpec(id)
    const cfg = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
    const out: Record<string, string> = {}
    for (const f of spec?.fields ?? []) {
        const v = cfg[f.key]
        if (f.type === 'checkbox') {
            out[f.key] = v === true ? 'true' : 'false'
        } else if (Array.isArray(v)) {
            // codeforces' initial_scores is a list. Joined for the one text input it
            // gets; split again on the way out.
            out[f.key] = v.join(', ')
        } else if (v === undefined || v === null) {
            out[f.key] = ''
        } else {
            out[f.key] = String(v)
        }
    }
    return out
}

/** The form's strings back into the JSON the server validates. */
export function configFromForm(id: string, form: Record<string, string>): Record<string, unknown> {
    const spec = formatSpec(id)
    const out: Record<string, unknown> = {}
    for (const f of spec?.fields ?? []) {
        const raw = form[f.key]
        if (f.type === 'checkbox') {
            out[f.key] = raw === 'true'
            continue
        }
        if (raw === undefined || raw.trim() === '') continue
        if (f.key === 'initial_scores') {
            const nums = raw.split(',').map((s) => Number(s.trim())).filter((n) => Number.isFinite(n))
            if (nums.length > 0) out[f.key] = nums
            continue
        }
        const n = Number(raw)
        if (Number.isFinite(n)) out[f.key] = n
    }
    return out
}
/** API response shapes generated from internal/model structs (JSON tags are the source
 * of truth; embedded structs are flattened as Go JSON does). Hand-maintained types for
 * responses not backed by internal/model structs follow the generated section.
 */

/**
 * Per-surface access. The vocabulary lives here rather than in lib/contestAccess.ts
 * because this file has no imports by design (it is generated-shaped), and the
 * lib re-exports these so there is exactly one definition of each.
 *
 * `open` is what an ABSENT key resolves to: anyone who can open the round. It is
 * separately named because a setter needs a way to opt one surface OUT of a
 * round-wide "participants" base, which clearing the row cannot express — a
 * cleared row inherits the base.
 */
export type AccessAudience = 'open' | 'participants' | 'public' | 'staff'

export type AccessSurface =
    | 'participate'
    | 'problem_list'
    | 'statement'
    | 'pdf'
    | 'submit'
    | 'standings'

/** `{surface: audience}`. An absent key means "inherit the base". */
export type AccessConfig = Partial<Record<AccessSurface, AccessAudience>>

export interface AIModel {
    id: string
    name: string
    endpoint: string
    api_key?: string
    has_api_key?: boolean
    model_name: string
    enabled: boolean
    priority: number
    description: string
    allow_private_network?: boolean
    structured_output_mode?: string
    request_timeout_seconds?: number
    max_output_tokens?: number
    capabilities?: Record<string, unknown>
    last_test_status?: string
    last_tested_at?: string | null
    created_at: string
    updated_at: string
}

export interface AIBrief {
    tags: string[]; rating: number; problem_type: string; testcase_count: number; has_subtasks: boolean
    additional_prompt?: string; primary_topics?: string[]; secondary_topics?: string[]; required_techniques?: string[]
    forbidden_techniques?: string[]; rating_min?: number; rating_max?: number; solver_mode?: 'solo' | 'team'
    team_size?: number; event_context?: 'practice' | 'online' | 'onsite'; format_profile?: 'general' | 'codeforces' | 'icpc' | 'oi'
    expected_solve_minutes?: number; audience_level?: string; insight_emphasis?: 'insight' | 'implementation' | 'balanced'
    story_tone?: string; constraint_guidance?: string
}
export interface AIValidationCheck { name: string; passed: boolean; message: string }
export interface AIValidationReport { valid: boolean; checks: AIValidationCheck[] }
export interface AIProposal {
    title?: string; description?: string; input_format?: string; output_format?: string; hint?: string
    sample_cases?: Array<{ input: string; output: string; explanation?: string }>; time_limit_ms?: number; memory_limit_kb?: number
    tags?: string[]; difficulty?: string; solution_code?: string; solution_language?: string; editorial_title?: string
    editorial_content?: string; editorial_approach?: string; time_complexity?: string; space_complexity?: string
    generator_code?: string; generator_language?: string; seeds?: number[]
    curated_cases?: Array<{ name: string; input: string; intent: string; score: number }>; difficulty_rationale?: Record<string, unknown>
}
export interface AIGenerationJob {
    id: string; actor_id: string; problem_id?: string; ai_model_id?: string; artifact_type: string
    status: 'queued'|'running'|'ready'|'validation_failed'|'failed'|'cancelled'|'applied'|'discarded'
    stage: string; progress: number; submitted_brief: AIBrief; normalized_brief: AIBrief
    proposal?: AIProposal; validation_report?: AIValidationReport; error_code?: string; error_message?: string
    prompt_tokens: number; completion_tokens: number; latency_ms: number
    created_at: string; started_at?: string | null; finished_at?: string | null; applied_at?: string | null
    cancel_requested_at?: string | null; heartbeat_at?: string | null
}
export interface AIBriefPreset { id: string; name: string; description: string; values: Partial<AIBrief> }

export interface APIKey {
    id: string
    user_id: string
    key_preview: string
    name: string
    description: string
    rate_limit: number
    is_active: boolean
    last_used_at?: string | null
    created_at: string
}

export interface Achievement {
    id: string
    code: string
    title: string
    description: string
    icon: string
    created_at: string
}

export interface AuditEntry {
    id: string
    actor_id?: string
    actor_name: string
    action: string
    target_type?: string
    target_id?: string
    detail: unknown
    ip?: string
    created_at: string
}

export interface AuthResponse {
    access_token?: string
    refresh_token?: string
    user?: User | null
    requires_2fa?: boolean
    challenge_id?: string
    /**
     * Where this session belongs, when the server knows better than the client.
     *
     * Set for a generated onsite team account, which exists for one round and
     * should land in it rather than the home page. Absent for every ordinary
     * sign-in, so a person who navigated to /login?next=… keeps their choice.
     */
    redirect_to?: string
}

export interface BalloonRequest {
    id: string
    contest_id: string
    submission_id: string
    user_id: string
    username: string
    /** Set for a generated onsite account: the team it was minted for. */
    team_name?: string
    institution?: string
    problem_id: string
    problem_index: string
    color: string
    dispatched: boolean
    created_at: string
}

export interface BatchUserRequest {
    team_name: string
    institution?: string
}

export interface BlogListItem {
    id: string
    user_id: string
    username: string
    title: string
    tags: string[]
    upvotes: number
    comment_count: number
    created_at: string
}

export interface BlogPost {
    id: string
    user_id: string
    username?: string
    title: string
    content: string
    tags: string[]
    upvotes: number
    downvotes: number
    comment_count: number
    is_pinned: boolean
    created_at: string
    updated_at: string
}

export interface BotAccount {
    id: string
    user_id: string
    platform: string
    platform_user: string
    api_key?: string
    api_secret?: string
    session_data?: Record<string, string>
    status: string
    rate_limit_rps: number
    last_used_at?: string | null
    consecutive_failures: number
    last_error_at?: string | null
    last_poll_at?: string | null
    proxy_url: string
    proxy_enabled: boolean
    created_at: string
}

export interface Clarification {
    id: string
    contest_id: string
    user_id: string
    username?: string
    problem_id?: string | null
    problem_idx?: string | null
    question: string
    answer?: string | null
    is_public: boolean
    answered_by?: string | null
    created_at: string
    updated_at: string
}

export interface Class {
    id: string
    organization_id: string
    org_name?: string
    name: string
    description: string
    invite_code: string
    created_by: string
    creator_name?: string
    student_count: number
    created_at: string
    updated_at: string
}

export interface ClassListItem {
    id: string
    organization_id: string
    name: string
    description: string
    student_count: number
    created_at: string
}

export interface ClassMember {
    class_id: string
    user_id: string
    username?: string
    role: string
    joined_at: string
}

export interface Comment {
    id: string
    user_id: string
    username?: string
    parent_type: string
    parent_id: string
    content: string
    upvotes: number
    created_at: string
    updated_at: string
}

export interface Contest {
    id: string
    display_id: number
    slug?: string
    title: string
    type: string
    format: string
    format_config?: unknown
    start_time: string
    end_time: string
    freeze_time?: string | null
    has_password: boolean
    visible: boolean
    description?: string
    registration_required: boolean
    registration_deadline?: string | null
    max_participants?: number | null
    division: number
    educational_config?: EducationalRoundConfig | null
    hack_phase_enabled: boolean
    hack_phase_start?: string | null
    hack_phase_end?: string | null
    team_size: number
    is_team_contest: boolean
    upsolving_enabled: boolean
    virtual_contest_enabled: boolean
    pdf_enabled: boolean
    statement_hidden: boolean
    /**
     * Who may reach each part of the round: {surface: audience}.
     *
     * An ABSENT key means "inherit", which is not the same as a stored default —
     * the map only ever records a deliberate override, so an empty object is a
     * round that behaves exactly as it did before this setting existed. Read it
     * through the helper in lib/contestAccess.ts rather than indexing directly, or
     * you will treat "inherit" and "open" as different things when they resolve
     * the same way.
     */
    access_config?: AccessConfig
    /**
     * Whether the round appears in the public contest catalog and in search.
     * Defaults to true on the server; every round that predates this field is
     * listed.
     *
     * NOT the same as `visible`, which means "may be opened" and is derived from
     * status by the server — editing a round can never publish it. An unlisted
     * round is still reachable by its direct link under its own access rules, and
     * is simply not enumerated. That pair is what makes this usable for an onsite
     * or invitation-only round, whose participants hold a URL.
     */
    listed?: boolean
    group_id?: string
    rating_calculated: boolean
    created_by: string
    created_at: string
    /** Review workflow: draft | pending | published | rejected. */
    status?: string
    review_note?: string
    /** is_rated existed in the schema since migration 000003 but was absent from
     * every Go struct and from this type, so a contest could not be unrated. */
    is_rated?: boolean
    rated_for_min?: number
    rated_for_max?: number
    /**
     * Who runs this round. Present only when the caller may read the round as
     * content: the server withholds the roster from a caller who may not, so an
     * absent key and an empty list are different answers and the About tab
     * renders neither as "no organizers".
     */
    judges?: ContestPermission[]
}

export interface ContestNotice {
    id: string
    contest_id: string
    content: string
    created_by: string
    username?: string
    created_at: string
}

/**
 * A manual correction to a contest standing.
 *
 * A signed delta layered over the computed result — the submissions keep the
 * verdicts they earned, and the reason is required so the correction can be
 * understood months later.
 */
export interface ContestAdjustment {
    id: string
    contest_id: string
    user_id: string
    username?: string
    /** The onsite team the account competes as, when it was minted for one. */
    team_name?: string
    scope: 'contest' | 'problem' | 'submission'
    problem_id?: string | null
    problem_index?: string | null
    submission_id?: string | null
    point_delta: number
    solved_delta: number
    penalty_delta: number
    reason: string
    created_by?: string | null
    created_at: string
}

/** The deltas applied to one scoreboard row, present only when corrected. */
export interface ScoreboardAdjustment {
    PointDelta: number
    SolvedDelta: number
    PenaltyDelta: number
}

export interface ContestPermission {
    contest_id: string
    user_id: string
    username?: string
    access_level: string
}

export interface ContestProblem {
    contest_id: string
    problem_id: string
    index: string
    title?: string
    slug?: string
    score: number
    sort_order: number
    /** How many participants in this round have solved it. Always present. */
    solved_count?: number
    /** Per-viewer progress. Absent when signed out, so "unsolved" and
     *  "we don't know who you are" stay distinguishable. */
    my_solved?: boolean
    my_attempted?: boolean
    my_submissions?: number
}

export interface ContestRankEntry {
    user_id: string
    username: string
    problems: Record<string, ProblemResult>
    total_solved: number
    total_penalty: number
    total_score: number
    last_ac_time?: string | null
}

export interface ContestRatingRequest {
    contest_id: string
}

export interface ContestRegistration {
    contest_id: string
    user_id: string
    username?: string
    registered_at: string
}

export interface CreateAPIKeyRequest {
    name: string
    description: string
    rate_limit?: number
}

export interface CreateAPIKeyResponse {
    api_key: APIKey | null
    secret: string
}

export interface CreateBlogRequest {
    title: string
    content: string
    tags?: string[]
}

export interface CreateClassRequest {
    name: string
    description: string
}

export interface CreateCommentRequest {
    parent_type: string
    parent_id: string
    content: string
}

export interface CreateContestRequest {
    title: string
    slug?: string
    type: string
    format: string
    format_config?: unknown
    division: number
    start_time: string
    end_time: string
    freeze_time?: string | null
    password: string | null
    description?: string
    visible?: boolean | null
    problem_ids: string[]
    pdf_enabled?: boolean | null
    statement_hidden?: boolean | null
    upsolving_enabled?: boolean | null
    virtual_contest_enabled?: boolean | null
    group_id?: string
    registration_required: boolean
    registration_deadline?: string | null
    max_participants?: number | null
}

export interface CreateEditorialRequest {
    problem_id: string
    contest_id?: string
    title: string
    content: string
    solution_code?: string
    solution_language?: string
    approach?: string
    time_complexity?: string
    space_complexity?: string
}

export interface CreateGroupRequest {
    name: string
    description: string
    is_public: boolean
    max_members?: number | null
    join_policy: string
}

export interface CreateGymRequest {
    contest_id: string
    difficulty_rating?: number | null
    category: string
    country?: string
    season?: string
    description: string
    is_public: boolean
}

export interface CreateNotificationRequest {
    user_id: string
    type: string
    title: string
    content: string
    link?: string
}

export interface CreateOrganizationRequest {
    name: string
    description: string
}

export interface CreateProblemEntryRequest {
    problem_id: string
    points: number
}

export interface CreateProblemRequest {
    slug: string
    title: string
    description: string
    input_format?: string
    output_format?: string
    constraints?: string
    hint?: string
    time_limit: number
    memory_limit: number
    difficulty: string
    tags?: string[]
    sample_cases?: SampleCase[]
    testcase_score?: TestCaseScore[]
    spj: boolean
    spj_language?: string
    spj_source_code?: string
    checker_type?: string
    float_epsilon?: number
    visible: boolean
    interactive: boolean
    interactor_language?: string
    interactor_source_code?: string
    scoring_mode?: string
    subtask_aggregation?: string
}

export interface CreateSectionRequest {
    title: string
    description: string
    problems: CreateProblemEntryRequest[]
}

export interface CreateTeamRequest {
    name: string
    description: string
    is_public: boolean | null
}

export interface CreateTrainingPlanRequest {
    title: string
    description: string
    organization_id?: string | null
    sections: CreateSectionRequest[]
}

export interface CreateWebhookRequest {
    url: string
    events: string[]
    description?: string
}

export interface Editorial {
    id: string
    problem_id: string
    problem_title?: string
    contest_id?: string | null
    user_id: string
    username?: string
    title: string
    content: string
    solution_code?: string
    solution_language?: string
    approach?: string
    time_complexity?: string
    space_complexity?: string
    is_official: boolean
    upvotes: number
    created_at: string
    updated_at: string
}

export interface EducationalRoundConfig {
    hack_phase_hours: number
    show_solutions: boolean
    allow_upsolving: boolean
    rated_for_divisions: number[]
}

export interface EmailVerificationToken {
    id: string
    user_id: string
    expires_at: string
    used: boolean
    created_at: string
}

export interface ForgotPasswordRequest {
    email: string
}

export interface Group {
    id: string
    name: string
    description: string
    is_public: boolean
    max_members?: number | null
    invite_code?: string
    join_policy: string
    created_by: string
    creator_name?: string
    member_count: number
    created_at: string
    updated_at: string
}

export interface GroupInviteRequest {
    username: string
}

export interface GroupListItem {
    id: string
    name: string
    description: string
    is_public: boolean
    member_count: number
    created_at: string
}

export interface GroupMember {
    group_id: string
    user_id: string
    username?: string
    role: string
    joined_at: string
}

export interface GroupPendingInvite {
    group_id: string
    group_name: string
    role: string
    joined_at: string
}

export interface GroupRespondRequest {
    user_id: string
    action: string
}

export interface GymContest {
    id: string
    contest_id: string
    contest_title?: string
    contest_slug?: string
    contest_display_id?: number
    difficulty_rating?: number | null
    category: string
    country?: string
    season?: string
    description: string
    is_public: boolean
    solve_count: number
    created_by: string
    creator_name?: string
    created_at: string
}

export interface Hack {
    id: string
    contest_id: string
    problem_id: string
    hacker_id: string
    hacker_username?: string
    defender_id: string
    defender_username?: string
    submission_id: string
    test_input: string
    expected_output?: string
    actual_output?: string
    status: string
    success?: boolean | null
    hacker_rating_change: number
    defender_rating_change: number
    created_at: string
    judged_at?: string | null
}

export interface HackRequest {
    contest_id: string
    problem_id: string
    submission_id: string
    test_input: string
}

export interface HackResult {
    hack_id: string
    status: string
    success: boolean
    expected_output?: string
    actual_output?: string
    message?: string
}

export interface JoinByCodeRequest {
    invite_code: string
}

export interface LanguageLimit {
    problem_id: string
    language_id: string
    time_limit_ms?: number | null
    memory_limit_kb?: number | null
}

export interface LoginRequest {
    username: string
    password: string
}

export interface Notification {
    id: string
    user_id: string
    type: string
    title: string
    content: string
    link?: string
    read: boolean
    created_at: string
}

export interface NotificationPreferences {
    user_id: string
    contest_announcements: boolean
    rating_changes: boolean
    hack_notifications: boolean
    group_activities: boolean
    email_digest: boolean
}

export interface OAuthLink {
    id: string
    user_id: string
    provider: string
    provider_user_id: string
    linked_at: string
}

/**
 * Response of a first-time OAuth callback: no session yet, because the account
 * is only created once the person has chosen a username.
 */
export interface OAuthSignupChallenge {
    requires_username: boolean
    signup_token: string
    suggested_username?: string
    provider?: string
    email?: string
}

/** GET /api/auth/oauth/username-available — the sign-up form's live hint. */
export interface OAuthUsernameCheck {
    available: boolean
    username?: string
    error?: string
}

export interface OnsiteBatchUser {
    id: string
    contest_id: string
    team_name: string
    institution?: string
    username: string
    password?: string
    is_used: boolean
    used_by?: string | null
    created_at: string
}

export interface Organization {
    id: string
    name: string
    description: string
    created_by: string
    creator_name?: string
    member_count: number
    created_at: string
    updated_at: string
}

export interface OrganizationListItem {
    id: string
    name: string
    description: string
    member_count: number
    created_at: string
}

export interface OrganizationMember {
    organization_id: string
    user_id: string
    username?: string
    role: string
    joined_at: string
}

export interface PasswordResetToken {
    id: string
    user_id: string
    expires_at: string
    used: boolean
    created_at: string
}

export interface PendingInvite {
    team_id: string
    team_name: string
    role: string
    joined_at: string
}

export interface PendingRemoteSubmission {
    id: string
    remote_id: string
    bot_id: string
    bot_slug: string
    status: string
    platform: string
}

export interface PlagiarismCheckRequest {
    contest_id: string
    threshold?: number
}

export interface PlagiarismPair {
    id: string
    report_id: string
    problem_id: string
    submission_a_id: string
    submission_b_id: string
    user_a_id: string
    user_b_id: string
    similarity: number
    status: PlagiarismPairStatus
    matched_lines: number
    created_at: string
    updated_at: string
}

export interface PlagiarismPairDetail {
    id: string
    report_id: string
    problem_id: string
    submission_a_id: string
    submission_b_id: string
    user_a_id: string
    user_b_id: string
    similarity: number
    status: PlagiarismPairStatus
    matched_lines: number
    created_at: string
    updated_at: string
    problem_title: string
    user_a_username: string
    user_b_username: string
    submission_a_lang: string
    submission_b_lang: string
}

export interface PlagiarismPairUpdateRequest {
    status: PlagiarismPairStatus
}

export interface PlagiarismReport {
    id: string
    contest_id: string
    status: PlagiarismReportStatus
    threshold: number
    total_pairs: number
    flagged_count: number
    error_message?: string
    created_by: string
    created_at: string
    completed_at?: string | null
}

export interface PlagiarismReportDetail {
    id: string
    contest_id: string
    status: PlagiarismReportStatus
    threshold: number
    total_pairs: number
    flagged_count: number
    error_message?: string
    created_by: string
    created_at: string
    completed_at?: string | null
    contest_title: string
    pairs: PlagiarismPairDetail[]
}

export interface PlanProgressSummary {
    total_problems: number
    completed_problems: number
    percentage: number
}

export interface PlatformStats {
    problems: number
    users: number
    submissions: number
}

export interface PrintRequest {
    id: string
    contest_id: string
    user_id: string
    username: string
    filename: string
    content: string
    status: string
    created_at: string
    updated_at: string
}

export interface Problem {
    id: string
    slug: string
    title: string
    description: string
    input_format?: string
    output_format?: string
    /** LaTeX/Markdown bounds block, rendered as its own Constraints section. */
    constraints?: string
    hint?: string
    sample_cases: SampleCase[]
    time_limit: number
    memory_limit: number
    difficulty: string
    /** Codeforces-scale rating; 0 means unrated. */
    rating: number
    tags?: string[]
    visible: boolean
    /** Review workflow: draft | pending | published | rejected. Publishing is
     * an admin decision, so `visible` is derived from this and is not
     * client-settable. */
    status?: string
    review_note?: string
    reviewed_at?: string
    /** "standard" | "interactive" | "output_only". */
    problem_type?: string
    testcase_score?: TestCaseScore[]
    spj: boolean
    spj_language?: string
    spj_source_code?: string
    spj_version?: string
    checker_type: string
    float_epsilon: number
    interactive: boolean
    interactor_language?: string
    interactor_source_code?: string
    scoring_mode: string
    subtask_aggregation: string
    language_limits?: LanguageLimit[]
    submission_count: number
    accepted_count: number
    source: string
    remote_id?: string
    ai_generated?: boolean
    created_by: string
    created_at: string
    updated_at: string
}

export interface ProblemI18n {
    problem_id: string
    language: string
    title?: string | null
    description?: string | null
    created_at: string
    updated_at: string
}

export interface ProblemListItem {
    id: string
    slug: string
    title: string
    difficulty: string
    rating: number
    tags: string[]
    submission_count: number
    accepted_count: number
    source: string
}

export interface ProblemPermission {
    problem_id: string
    user_id: string
    username?: string
    access_level: string
}

export interface ProblemResult {
    solved: boolean
    attempts: number
    time: number
    score: number
    pending: number
}

export interface ProblemStats {
    total_submissions: number
    accepted_submissions: number
    acceptance_rate: number
    unique_solvers: number
    average_attempts: number
    language_distribution: Record<string, number>
    difficulty_estimate: number
}

export interface ProblemWithSamples {
    id: string
    slug: string
    title: string
    description: string
    input_format?: string
    output_format?: string
    hint?: string
    sample_cases: SampleCase[]
    time_limit: number
    memory_limit: number
    difficulty: string
    tags?: string[]
    visible: boolean
    /** Review workflow: draft | pending | published | rejected. Publishing is
     * an admin decision, so `visible` is derived from this and is not
     * client-settable. */
    status?: string
    review_note?: string
    reviewed_at?: string
    /** "standard" | "interactive" | "output_only". */
    problem_type?: string
    testcase_score?: TestCaseScore[]
    spj: boolean
    spj_language?: string
    spj_source_code?: string
    spj_version?: string
    checker_type: string
    float_epsilon: number
    interactive: boolean
    interactor_language?: string
    interactor_source_code?: string
    scoring_mode: string
    subtask_aggregation: string
    language_limits?: LanguageLimit[]
    submission_count: number
    accepted_count: number
    source: string
    remote_id?: string
    ai_generated?: boolean
    created_by: string
    created_at: string
    updated_at: string
    index: string
}

export interface PublicProfile {
    id: string
    username: string
    email?: string
    rating: number
    rating_change: number
    contests_played: number
    problems_solved: number
    bio?: string
    avatar_url?: string
    first_name?: string
    last_name?: string
    country?: string
    city?: string
    organization?: string
    github_url?: string
    show_email: boolean
    show_tags: boolean
    created_at: string
}

export interface RankingEntry {
    id: string
    username: string
    rating: number
    rating_change: number
    contests_played: number
    country?: string
}

export interface RatingChange {
    user_id: string
    username: string
    old_rating: number
    new_rating: number
    rating_change: number
    rank: number
    color: string
}

export interface RatingHistory {
    id: string
    user_id: string
    contest_id: string
    old_rating: number
    new_rating: number
    rank: number
    rating_change: number
    created_at: string
}

export interface RecommendationsResponse {
    progression: ProblemListItem[]
    weak_tags: WeakTagsRecommendations
    hybrid: ProblemListItem[]
}

export interface RegisterRequest {
    username: string
    email: string
    password: string
}

export interface RemoteLanguage {
    id: string
    platform: string
    local_id: string
    remote_id: string
    display_name: string
    enabled: boolean
    sort_order: number
    inline_comment_prefix: string
}

export interface RemoteLanguageConfig {
    platform: string
    languages: RemoteLanguage[]
}

export interface ResetPasswordRequest {
    token: string
    new_password: string
}

export interface SampleCase {
    input: string
    output: string
    explanation?: string
}

export interface SetterApplication {
    user_id: string
    username: string
    status: string
    reason: string
    created_at: string
}

export interface Submission {
    id: string
    problem_id: string
    user_id: string
    username?: string
    contest_id?: string
    language: string
    source_code?: string
    submission_type: string
    code_size: number
    status: SubmissionStatus
    score: number
    time_used: number
    memory_used: number
    compile_output?: string
    /** The judge's own failure text for a System Error; admins/setters only. */
    judge_error?: string
    judge_result?: TestCaseResult[]
    judged_by: string
    remote_id?: string
    remote_url?: string
    bot_id?: string
    bot_slug?: string
    is_remote: boolean
    remote_oj?: string
    is_upsolving?: boolean
    /** Filled by the submission listings so a status table can name the problem. */
    problem_slug?: string
    problem_title?: string
    /** The round's letter (A, B, …), filled by the contest-scoped listings. */
    problem_index?: string
    /**
     * Per-viewer capabilities. Voiding and rejudging come from the same
     * accountability rule the API enforces, so a row offers one only where it
     * would offer the other.
     */
    can_view?: boolean
    can_rejudge?: boolean
    can_skip?: boolean
    created_at: string
    judged_at?: string | null
}

export interface SubmitRequest {
    problem_id: string
    language: string
    source_code: string
    contest_id?: string
    submission_type?: string
}

export interface SystemSetting {
    key: string
    value: unknown
    description: string
    updated_at: string
    updated_by?: string | null
}

export interface TOTPSecret {
    user_id: string
    enabled: boolean
    enabled_at?: string | null
    created_at: string
}

export interface Team {
    id: string
    name: string
    description: string
    avatar_url?: string
    rating: number
    max_rating: number
    contest_count: number
    member_count: number
    created_by: string
    creator_name?: string
    is_public: boolean
    created_at: string
    updated_at: string
}

export interface TeamContest {
    id: string
    team_id: string
    contest_id: string
    contest_title?: string
    rank?: number | null
    score: number
    rating_change: number
    participated_at: string
}

export interface TeamInviteRequest {
    username: string
}

export interface TeamListItem {
    id: string
    name: string
    rating: number
    member_count: number
    is_public: boolean
    created_at: string
}

export interface TeamMember {
    team_id: string
    user_id: string
    username?: string
    role: string
    joined_at: string
}

export interface TeamRegistration {
    id: string
    contest_id: string
    team_id: string
    team_name?: string
    registered_at: string
}

export interface TeamRespondRequest {
    user_id: string
    action: string
}

export interface TestCaseResult {
    case_name: string
    status: SubmissionStatus
    time: number
    memory: number
    score: number
    detail?: string
}

export interface TestCaseScore {
    input_name: string
    output_name: string
    score: number
    subtask_id?: number
    subtask_score?: number
}

/** One file in a problem's testdata directory. */
export interface TestcaseFile {
    name: string
    size: number
    /** Derived from the extension on the server. */
    role: 'input' | 'output' | 'other'
    /** Counterpart file sharing the stem, when one exists. */
    pair?: string
}

export interface TestcaseFileContent {
    name: string
    size: number
    content: string
    /** Non-UTF-8 bytes: download-only, the editor cannot represent them. */
    binary: boolean
}

export interface TrainingPlan {
    id: string
    title: string
    description: string
    organization_id?: string | null
    org_name?: string
    is_public: boolean
    created_by: string
    creator_name?: string
    section_count: number
    problem_count: number
    enrolled_count: number
    created_at: string
    updated_at: string
}

export interface TrainingPlanDetail {
    id: string
    title: string
    description: string
    organization_id?: string | null
    org_name?: string
    is_public: boolean
    created_by: string
    creator_name?: string
    section_count: number
    problem_count: number
    enrolled_count: number
    created_at: string
    updated_at: string
    sections: TrainingPlanSection[]
    enrolled: boolean
    progress?: PlanProgressSummary | null
}

export interface TrainingPlanEnrollment {
    plan_id: string
    user_id: string
    username?: string
    enrolled_at: string
}

export interface TrainingPlanProblem {
    id: string
    section_id: string
    problem_id: string
    sort_order: number
    points: number
}

export interface TrainingPlanProgress {
    plan_id: string
    user_id: string
    problem_id: string
    completed: boolean
    completed_at?: string | null
}

export interface TrainingPlanSection {
    id: string
    plan_id: string
    title: string
    description: string
    sort_order: number
    problems?: TrainingPlanProblem[]
}

export interface UpdateGroupRequest {
    name: string
    description: string
    is_public: boolean | null
    max_members?: number | null
    join_policy: string
}

export interface UpdatePasswordRequest {
    current_password: string
    new_password: string
}

export interface UpdateProfileRequest {
    first_name: string
    last_name: string
    country: string
    city: string
    organization: string
    github_url: string
    bio: string
    avatar_url: string
    show_email: boolean | null
    show_tags: boolean | null
}

export interface UpdateTeamRequest {
    name: string
    description: string
    avatar_url: string
    is_public: boolean | null
}

export interface User {
    id: string
    username: string
    email?: string
    role: string
    is_bot: boolean
    /** Account still holds a seeded/admin-issued password; only the profile and
     *  password endpoints are reachable until it is changed. */
    must_change_password?: boolean
    created_at: string
    updated_at: string
}

export interface UserAchievement {
    achievement_id: string
    code: string
    title: string
    description: string
    icon: string
    awarded_at: string
}

export interface UserProblemStats {
    problems_solved: number
    total_submissions: number
    acceptance_rate: number
    favorite_language: string
    streak_days: number
}

export interface UserProfile {
    user_id: string
    /** The owner's own account email; never returned for other users. */
    email: string
    created_at: string
    rating: number
    max_rating: number
    contest_count: number
    problems_solved: number
    submissions: number
    bio?: string
    avatar_url?: string
    first_name?: string
    last_name?: string
    country?: string
    city?: string
    organization?: string
    github_url?: string
    show_email: boolean
    show_tags: boolean
}

export interface VJSubmitRequest {
    problem_id: string
    problem_remote_id: string
    remote_oj: string
    language: string
    source_code: string
}

export interface VirtualContest {
    id: string
    original_contest_id: string
    user_id: string
    username?: string
    contest_title?: string
    started_at: string
    ended_at?: string | null
    duration_minutes: number
    status: string
    created_at: string
}

export interface VirtualStatus {
    is_active: boolean
    virtual_id?: string
    started_at?: string | null
    ends_at?: string | null
    remaining_minutes: number
}

export interface Vote {
    user_id: string
    target_type: string
    target_id: string
    value: number
    created_at: string
}

export interface WeakTagsRecommendations {
    tags: string[]
    problems: ProblemListItem[]
}

export interface Webhook {
    id: string
    user_id: string
    url: string
    events: string[]
    is_active: boolean
    description: string
    created_at: string
    updated_at: string
}

export type PlagiarismPairStatus = string

export type PlagiarismReportStatus = string

export type SubmissionStatus = string

// ---- hand-maintained response shapes (no internal/model struct) ----

/** Auth response whose tokens are guaranteed present (register / 2FA verify). */
export interface AuthSessionResponse {
    access_token: string
    refresh_token: string
    user?: User | null
}

/** GET /api/activity item (internal/store/postgres.ActivityItem). */
export interface ActivityItem {
    id: string
    type: 'ac' | 'rating' | 'comment'
    username: string
    title: string
    summary?: string
    link?: string
    created_at: string
}

/** GET /api/contests/{id} — GetByID wraps the contest with its problem set.
 *
 * registration_count / is_registered are now sent by GetByID, and both are 0/false
 * on a round that does not require registration (there is no roster to count).
 * They are still optional here, because the handler treats its registration store
 * as an optional collaborator: a build without one omits them and the page's
 * `?? 0` / `?? false` fallback is what it renders. They were read with those
 * fallbacks for a long time against a handler that sent neither, which is why the
 * participant count was permanently zero. */
export interface ContestDetailResponse {
    contest: Contest & { is_registered?: boolean; registration_count?: number }
    problems: ContestProblem[]
    /** True when the viewer runs this round: admin, creator, manager, judge or
     * tester. A judge of one contest is an ordinary user elsewhere, so this
     * cannot be derived from the session role. */
    is_staff?: boolean
    /** The narrower privilege: admin, creator, manager or judge — not tester. */
    is_judge?: boolean
}

/** GET /api/contests/{id}/scoreboard (view=…&page=…) and /standings. */
export interface ProblemCell {
    solved: boolean
    attempts: number
    time: number
    score: number
    pending: number
}
export interface ScoreboardEntry {
    rank: number
    user_id: string
    username: string
    /** Set for a generated onsite account: the team it was minted for. */
    team_name?: string
    institution?: string
    rating?: number
    total_solved: number
    total_penalty: number
    total_score: number
    problems: Record<string, ProblemCell>
}
export interface ProblemMeta {
    problem_id: string
    index: string
    score: number
}
export interface ContestInfo {
    id: string
    title: string
    format?: string
    status?: string
    start_time?: string
    end_time?: string
    is_rated?: boolean
    rating_calculated?: boolean
    is_team_contest?: boolean
}
export interface PaginationInfo {
    page: number
    limit: number
    total: number
    total_pages: number
}
export interface ScoreboardData {
    entries: ScoreboardEntry[]
    problems: ProblemMeta[]
    frozen: boolean
    contest: ContestInfo
    is_judge: boolean
    can_see_judge: boolean
    pagination: PaginationInfo
}

/**
 * GET /api/contests/{id}/submissions — judge-aware wrapper.
 *
 * The contest listing returns the same row shape as the status page and the
 * setter listing, so the three surfaces can render one table.
 */
export interface SubmissionRow {
    id?: string
    created_at: string
    status: string
    language?: string
    problem_id?: string
    /** The round's letter, which identifies a problem inside a contest. */
    problem_index?: string
    user_id?: string
    username?: string
    time_used?: number
    memory_used?: number
    score?: number
    judge_error?: string
    can_view?: boolean
    can_rejudge?: boolean
    can_skip?: boolean
}
export interface ContestSubmissionsResponse {
    data: SubmissionRow[]
    total?: number
    is_judge?: boolean
}

/** Rating change rows returned by /api/rating/user/{id} and /api/rating/contest/{id}. */
/**
 * What a rating operation did. `contests` is how many rounds were replayed, which
 * is the number an operator needs after a dispute: the answer is rarely "one",
 * because each round's rating is folded onto the previous one.
 */
export interface RatingRunResponse {
    run?: { id: string; kind: string; created_at: string }
    contests?: number
    participants?: number
    rated?: number
    changes?: unknown[]
    team_rating_error?: string
}

/** One entrant disqualified from one round. The round still counts for the rest. */
export interface RatingExclusion {
    user_id: string
    contest_id: string
    reason: string
    created_at: string
}

/** One payout: which round, who ran it, what kind, and why. */
export interface RatingRunEntry {
    id: string
    contest_id: string
    actor_id?: string
    actor_name: string
    kind: string
    reason: string
    created_at: string
}

export interface RatingHistoryEntry {
    id: string
    user_id: string
    contest_id: string
    old_rating: number
    new_rating: number
    rank: number
    rating_change: number
    created_at: string
}

/** GET /api/virtual/status — handler adds original_contest_id; timestamps are
 * always present at runtime despite omitempty on the model. */
export interface VirtualStatusResponse extends VirtualStatus {
    ends_at: string
    started_at: string
    original_contest_id: string
}

/** GET /api/languages — one judge-selectable language, as the server sees it. */
export interface PublicLanguage {
    key: string
    name: string
    extensions: string[]
}

/** Admin language config rows (YAML-backed, not DB models). */
export interface LanguageConfig {
    name: string
    key: string
    compile: string
    runtime: string
    copy_in: Record<string, string>
    time_limit_multiplier: number
    memory_limit_multiplier: number
    seccomp_rule: string
    extensions: string[]
    mono: boolean
    file_path: string
}
export interface DetectedTool { name: string; path: string; version?: string }
export interface TestResult { status?: string; key?: string; message?: string }

/** GET /api/admin/backups row. */
export interface BackupFile {
    filename: string
    size: number
    created_at: string
    type: string
}

/** GET /api/gym/{id} detail row. */
export interface GymData {
    contest_title: string
    description?: string
    category?: string
    difficulty_rating?: number
    solve_count: number
    country?: string
    contest_slug?: string
    contest_display_id?: number
    contest_id: string
}

/** Hackable submission rows for GET /api/hacks/hackable/{contestId}/{problemId}. */
export interface HackSubmission { id: string; language: string; user_id: string }

/** GET /api/contests/{id}/stats (internal/store/postgres.ContestStats). */
export interface ContestProblemStats {
    problem_id: string
    index: string
    title: string
    total_submissions: number
    accepted: number
    solve_rate: number
}
export interface ContestStats {
    total_participants: number
    total_submissions: number
    accepted_submissions: number
    problems: ContestProblemStats[]
    languages: Record<string, number>
    verdicts: Record<string, number>
}

// ── Feature gates (internal/feature) ─────────────────────────────────────────

/**
 * One admin decision about one feature. Mirrors internal/feature.Policy.
 * The zero value is "on for everybody", so an unconfigured feature is open.
 */
export interface FeaturePolicy {
    enabled: boolean
    /** Empty/absent means every role. */
    roles?: string[]
    /** 0 means unbounded. */
    min_rating?: number
    max_rating?: number
    /** Shown to a user who is denied access. */
    message?: string
}

/** One entry of GET /api/features, already resolved for the caller. */
export interface ResolvedFeature {
    key: string
    label: string
    description: string
    category: string
    default_policy: FeaturePolicy
    requires_login: boolean
    /** Whether the backend rejects requests, or the gate only hides UI. */
    api_enforced: boolean
    policy: FeaturePolicy
    allowed: boolean
    reason?: 'disabled' | 'login_required' | 'role' | 'rating'
    message?: string
}

// ── Sign-in methods (internal/authmethod) ────────────────────────────────────

/**
 * What the login and register forms may offer. Mirrors
 * authmethod.Availability: an OAuth provider is only true when it is both
 * switched on by an admin AND this deployment has credentials for it.
 */
export interface AuthMethods {
    password_registration: boolean
    google: boolean
    github: boolean
}

/** GET /api/admin/auth-methods: the switches plus why one may be unusable. */
export interface AdminAuthMethods {
    settings: { password_registration: boolean; google: boolean; github: boolean }
    /** Whether the deployment holds credentials. A switch can be on but dead. */
    configured: { google: boolean; github: boolean }
    default: { password_registration: boolean; google: boolean; github: boolean }
    customized: boolean
}

/** One entry of GET /api/admin/features. */
export interface AdminFeature {
    key: string
    label: string
    description: string
    category: string
    default_policy: FeaturePolicy
    requires_login: boolean
    api_enforced: boolean
    policy: FeaturePolicy
    /** True when a stored policy differs from the registry default. */
    customized: boolean
}

// ── Admin user management (GET /api/admin/users) ─────────────────────────────

/** One row of the admin users table. Mirrors model.AdminUserRow. */
export interface AdminUserRow {
    id: string
    username: string
    email: string
    role: string
    is_bot: boolean
    email_verified: boolean
    must_change_password: boolean
    /** Present only for a suspended account; the timestamp is what an appeal is judged against. */
    banned_at?: string
    ban_reason?: string
    rating: number
    problems_solved: number
    submissions: number
    created_at: string
    /** Absent until the account signs in for the first time. */
    last_login_at?: string
}

/** Single-user view (GET /api/admin/users/{id}). Mirrors model.AdminUserDetail. */
export interface AdminUserDetail extends AdminUserRow {
    max_rating: number
    contest_count: number
    country?: string
    organization?: string
    github_url?: string
    bio?: string
    totp_enabled: boolean
    /** Unexpired refresh tokens — the observable "someone is signed in". */
    active_sessions: number
}

export type AdminUserStatus = 'active' | 'banned' | 'unverified' | 'must_change_password'
export type AdminUserSort = 'created' | 'username' | 'rating' | 'submissions' | 'last_login'

/** Query for the admin users listing. Empty fields mean "no constraint". */
export interface AdminUserQuery {
    search?: string
    role?: string
    status?: AdminUserStatus | ''
    sort?: AdminUserSort | ''
    order?: 'asc' | 'desc'
    offset?: number
    limit?: number
}

export type AdminBulkAction = 'ban' | 'unban' | 'verify_email' | 'unverify_email' | 'set_role'

/** Per-id outcome of a bulk action; failures are reported, not fatal. */
export interface AdminBulkResult {
    id: string
    ok: boolean
    error?: string
}

export interface AdminBulkResponse {
    results: AdminBulkResult[]
    succeeded: number
    failed: number
}

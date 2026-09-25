/** Shared shapes for membership/invite data consumed across pages. */

export interface ClassMember {
    user_id: string
    username?: string
    role: string
    joined_at?: string
}

export interface ClassInfo {
    id: string
    name: string
    description?: string | null
    invite_code?: string
    org_name?: string
    organization_id?: string
    student_count?: number
}

export interface TeamInvite {
    team_id: string
    team_name?: string
    role: string
}

export interface GroupInvite {
    group_id: string
    group_name?: string
    role: string
}

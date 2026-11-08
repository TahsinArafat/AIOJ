/**
 * Remote online judges, mirroring `model.RemotePlatforms` on the backend.
 *
 * "Remote" is an explicit allowlist, not "anything that is not local". AI
 * generated problems carry the source `ai` and are judged by the local worker;
 * treating them as remote asked the API for a platform called "ai" that does
 * not exist — wrong language list on the problem page, and submissions that
 * never reached the judge.
 *
 * Adding a platform means adding it here and to the Go list.
 */
const REMOTE_PLATFORMS = ['codeforces', 'atcoder', 'cses', 'toph', 'qoj'] as const

export function isRemotePlatform(source: string | null | undefined): source is string {
    return !!source && (REMOTE_PLATFORMS as readonly string[]).includes(source)
}

/** Human labels for every problem source, including the non-remote ones. */
export const PLATFORM_LABEL: Record<string, string> = {
    local: 'AIOJ',
    ai: 'AI',
    codeforces: 'Codeforces',
    atcoder: 'AtCoder',
    cses: 'CSES',
    toph: 'Toph',
    qoj: 'QOJ',
}

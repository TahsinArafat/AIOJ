/** Stringify an unknown thrown value into a user-facing message. */
export function errorMessage(e: unknown): string {
    if (e instanceof Error) return e.message
    if (typeof e === 'string') return e
    return String(e)
}

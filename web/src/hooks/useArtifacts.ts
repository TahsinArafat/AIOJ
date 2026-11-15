import { useMemo } from 'react'
import { api } from '../lib/api'
import { ArtifactScope } from '../lib/aiArtifacts'
import { useAsyncData } from './useAsyncData'
import type { AIArtifactSpec } from '../types/api'

/**
 * The artifact catalog, fetched once per surface that needs it.
 *
 * The scopes are server policy (internal/model/ai_artifact.go): the same table
 * validates a brief and decides what goes into the prompt. Fetching it means the
 * form renders the fields the server will actually read, instead of restating
 * the rules in a second place that can disagree with the one that decides.
 *
 * The request is made by the signed-in setter and the route is behind the same
 * feature gate as the rest of the AI surface, so a deployment with AI off has no
 * catalog — and `error` is what says so. A caller renders the form's error state
 * rather than an empty picker in that case.
 */
export function useArtifacts() {
    return useAsyncData(() => api.ai.artifacts(), [])
}

/**
 * The scope for one artifact, with the questions the brief form asks of it.
 *
 * `spec` is undefined while the catalog is in flight or when the artifact is not
 * in it. ArtifactScope answers "accepts everything, requires nothing" in that
 * case, so a slow catalog shows the full form briefly rather than an empty one —
 * and the run is rejected server-side if the artifact was genuinely unknown.
 */
export function useArtifactScope(spec: AIArtifactSpec | undefined): ArtifactScope {
    return useMemo(() => new ArtifactScope(spec), [spec])
}

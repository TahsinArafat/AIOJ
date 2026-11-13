import { Badge } from '../ui/Badge'
import { statusLabel, statusTone } from '../../lib/aiJobs'

/**
 * One status pill for every AI surface.
 *
 * Was four copies. The copy that mattered was the dot: "queued" and "running" get
 * a pulsing indicator, and a surface that forgot it left a running job looking
 * static next to a live one.
 */
export default function AIJobStatusBadge({ status }: { status: string }) {
    return (
        <Badge tone={statusTone(status)} dot={status === 'running' || status === 'queued'}>
            {statusLabel(status)}
        </Badge>
    )
}

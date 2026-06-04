// web/src/components/RatingGraph.tsx
import { useState } from 'react'
import { getRatingColor } from '../lib/rating'

interface RatingEntry {
  id: string
  new_rating: number
  old_rating: number
  rating_change: number
  contest_id: string
  rank?: number
  created_at: string
}

interface TooltipState {
  x: number
  y: number
  entry: RatingEntry
  index: number
}

interface Props {
  history: RatingEntry[]
  width?: number
  height?: number
}

const BANDS = [
  { min: 0,    max: 1200, color: '#80808015', label: 'Novice' },
  { min: 1200, max: 1400, color: '#00800015', label: 'Apprentice' },
  { min: 1400, max: 1600, color: '#03A89E15', label: 'Adept' },
  { min: 1600, max: 1900, color: '#0000FF15', label: 'Elite' },
  { min: 1900, max: 2100, color: '#AA00AA15', label: 'Champion' },
  { min: 2100, max: 2300, color: '#FFD70015', label: 'Master' },
  { min: 2300, max: 2400, color: '#FF8C0015', label: 'Grandmaster' },
  { min: 2400, max: 2600, color: '#FF8C0015', label: 'Titan' },
  { min: 2600, max: 2900, color: '#FF000015', label: 'Immortal' },
  { min: 2900, max: 4000, color: '#FF000015', label: 'Apex' },
]

export default function RatingGraph({ history, width = 600, height = 200 }: Props) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  if (history.length === 0) {
    return (
      <div className="flex items-center justify-center h-32 text-gray-400 dark:text-gray-500 text-sm">
        No contest history yet
      </div>
    )
  }

  const PAD = { top: 16, right: 20, bottom: 32, left: 48 }
  const W = width - PAD.left - PAD.right
  const H = height - PAD.top - PAD.bottom

  const ratings = history.map(h => h.new_rating)
  const minR = Math.max(0, Math.min(...ratings) - 100)
  const maxR = Math.max(...ratings) + 100

  const xScale = (i: number) => history.length === 1 ? W / 2 : (i / (history.length - 1)) * W
  const yScale = (r: number) => H - ((r - minR) / (maxR - minR)) * H

  const points = history.map((e, i) => `${xScale(i)},${yScale(e.new_rating)}`).join(' ')

  const yTicks = 4
  const yTickVals = Array.from({ length: yTicks + 1 }, (_, i) =>
    Math.round(minR + (i / yTicks) * (maxR - minR))
  )

  return (
    <div className="relative w-full overflow-x-auto">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full"
        style={{ minWidth: Math.max(width, history.length * 24) }}
        onMouseLeave={() => setTooltip(null)}
      >
        <g transform={`translate(${PAD.left},${PAD.top})`}>
          {BANDS.map(band => {
            const bandTop = yScale(Math.min(band.max, maxR))
            const bandBot = yScale(Math.max(band.min, minR))
            if (bandBot <= bandTop) return null
            return (
              <rect
                key={band.label}
                x={0} y={bandTop}
                width={W} height={bandBot - bandTop}
                fill={band.color}
              />
            )
          })}

          {yTickVals.map(v => (
            <g key={v}>
              <line
                x1={0} y1={yScale(v)} x2={W} y2={yScale(v)}
                stroke="currentColor" strokeOpacity={0.1} strokeWidth={1}
              />
              <text
                x={-6} y={yScale(v)} textAnchor="end"
                dominantBaseline="middle"
                fontSize={10} fill="currentColor" fillOpacity={0.5}
              >
                {v}
              </text>
            </g>
          ))}

          {history.map((_e, i) => {
            const step = Math.max(1, Math.floor(history.length / 8))
            if (i % step !== 0 && i !== history.length - 1) return null
            return (
              <text
                key={i}
                x={xScale(i)} y={H + 20}
                textAnchor="middle"
                fontSize={9} fill="currentColor" fillOpacity={0.45}
              >
                {i + 1}
              </text>
            )
          })}

          <polyline
            points={points}
            fill="none"
            stroke="#6366f1"
            strokeWidth={2}
            strokeLinejoin="round"
            strokeLinecap="round"
          />

          {history.map((entry, i) => {
            const cx = xScale(i)
            const cy = yScale(entry.new_rating)
            const col = getRatingColor(entry.new_rating)
            return (
              <circle
                key={entry.id || i}
                cx={cx} cy={cy} r={4}
                fill={col.hex}
                stroke="white"
                strokeWidth={1.5}
                style={{ cursor: 'pointer' }}
                onMouseEnter={() => {
                  setTooltip({
                    x: cx + PAD.left,
                    y: cy + PAD.top,
                    entry,
                    index: i,
                  })
                }}
              />
            )
          })}
        </g>

        {tooltip && (() => {
          const tx = Math.min(tooltip.x + 10, width - 150)
          const ty = Math.max(tooltip.y - 70, 4)
          const d = tooltip.entry
          const sign = d.rating_change >= 0 ? '+' : ''
          const changeColor = d.rating_change > 0 ? '#22c55e' : d.rating_change < 0 ? '#ef4444' : '#9ca3af'
          return (
            <g>
              <rect x={tx} y={ty} width={140} height={60} rx={6}
                fill="white" stroke="#e5e7eb" strokeWidth={1}
                filter="drop-shadow(0 2px 4px rgba(0,0,0,0.1))"
              />
              <text x={tx + 8} y={ty + 16} fontSize={11} fontWeight="600" fill="#111">
                Rating: {d.new_rating}
              </text>
              <text x={tx + 8} y={ty + 30} fontSize={10} fill={changeColor}>
                {sign}{d.rating_change}
              </text>
              <text x={tx + 8} y={ty + 44} fontSize={9} fill="#6b7280">
                {new Date(d.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
              </text>
              {d.rank && (
                <text x={tx + 8} y={ty + 57} fontSize={9} fill="#6b7280">
                  Rank #{d.rank}
                </text>
              )}
            </g>
          )
        })()}
      </svg>
    </div>
  )
}

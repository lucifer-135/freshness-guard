function bandColor(score: number): string {
  if (score >= 60) return 'var(--status-current)'
  if (score >= 21) return 'var(--status-unverified)'
  return 'var(--status-outdated)'
}

// ScoreRing draws a freshness score as a partly drained ring (DESIGN.md section 2).
export function ScoreRing({ score, size = 'md' }: { score: number | null; size?: 'sm' | 'md' }) {
  const px = size === 'sm' ? 28 : 40
  const stroke = size === 'sm' ? 3 : 4
  const r = (px - stroke) / 2
  const circumference = 2 * Math.PI * r
  const filled = score === null ? 0 : (Math.max(0, Math.min(score, 100)) / 100) * circumference
  return (
    <span
      role="img"
      aria-label={score === null ? 'No freshness score' : `Freshness score ${score} out of 100`}
      className="relative inline-flex shrink-0 items-center justify-center"
      style={{ width: px, height: px }}
    >
      <svg width={px} height={px} viewBox={`0 0 ${px} ${px}`} className="-rotate-90" aria-hidden="true">
        <circle cx={px / 2} cy={px / 2} r={r} fill="none" stroke="var(--surface-3)" strokeWidth={stroke} />
        {score !== null && (
          <circle
            cx={px / 2}
            cy={px / 2}
            r={r}
            fill="none"
            stroke={bandColor(score)}
            strokeWidth={stroke}
            strokeLinecap="round"
            strokeDasharray={`${filled} ${circumference}`}
          />
        )}
      </svg>
      <span className={`absolute font-mono tabular-nums ${size === 'sm' ? 'text-[10px]' : 'text-xs'}`} aria-hidden="true">
        {score ?? '–'}
      </span>
    </span>
  )
}

// ScoreFigure is the score as a figure tinted by its band, for lists and tables,
// where a ring on every row would stand in for the number instead of showing it.
export function ScoreFigure({ score }: { score: number | null }) {
  return (
    <span
      className="font-mono tabular-nums"
      style={{ color: score === null ? 'var(--text-subtle)' : bandColor(score) }}
      aria-label={score === null ? 'No freshness score' : `Freshness score ${score} out of 100`}
    >
      {score ?? '–'}
    </span>
  )
}

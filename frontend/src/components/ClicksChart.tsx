import { weekdayLabel } from '../format'
import type { DailyClick } from '../types'

interface ClicksChartProps {
  days: DailyClick[]
}

// The "Clicks these past 7 days" bar chart, plain divs per the design: one
// flex column per backend-provided day (already zero-filled and ordered),
// count label above each bar, weekday label below. Rendered by the Stats page.
function ClicksChart({ days }: ClicksChartProps) {
  const max = Math.max(...days.map((d) => d.count), 1)

  return (
    <div className="chart">
      <div className="chart-title">Clicks these past 7 days</div>
      <div className="chart-track">
        {days.map((d, i) => {
          const isMax = d.count === max
          const isToday = i === days.length - 1
          // Max day at full opacity, today slightly raised, rest recede — max wins.
          const opacity = isMax ? 1 : isToday ? 0.72 : 0.35
          return (
            <div key={d.date} className="chart-col">
              <span className={`chart-count${isMax ? ' chart-count-max' : ''}`}>
                {d.count.toLocaleString('en-US')}
              </span>
              <div
                className="chart-bar"
                style={{ height: `${(d.count / max) * 100}%`, opacity }}
              />
            </div>
          )
        })}
      </div>
      <div className="chart-labels">
        {days.map((d) => (
          <div key={d.date} className="chart-label">
            {weekdayLabel(d.date)}
          </div>
        ))}
      </div>
    </div>
  )
}

export default ClicksChart

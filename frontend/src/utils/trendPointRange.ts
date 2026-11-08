export interface TrendPointRange {
  start: Date
  end: Date
}

type LocalParts = {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  second: number
}

const makeFormatter = (timeZone: string) => new Intl.DateTimeFormat('en-US', {
  timeZone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hourCycle: 'h23',
})

const readParts = (formatter: Intl.DateTimeFormat, date: Date): LocalParts => {
  const parts = Object.fromEntries(formatter.formatToParts(date).map(({ type, value }) => [type, value]))
  return {
    year: Number(parts.year),
    month: Number(parts.month),
    day: Number(parts.day),
    hour: Number(parts.hour),
    minute: Number(parts.minute),
    second: Number(parts.second),
  }
}

const localPartsAsUTC = (parts: LocalParts): number =>
  Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second)

const sameParts = (a: LocalParts, b: LocalParts): boolean =>
  a.year === b.year && a.month === b.month && a.day === b.day &&
  a.hour === b.hour && a.minute === b.minute && a.second === b.second

const toInstant = (parts: LocalParts, formatter: Intl.DateTimeFormat): Date => {
  const wallTime = localPartsAsUTC(parts)
  const offsets = new Set<number>()
  for (let deltaHours = -48; deltaHours <= 48; deltaHours += 6) {
    const candidate = new Date(wallTime + deltaHours * 60 * 60 * 1000)
    offsets.add(localPartsAsUTC(readParts(formatter, candidate)) - candidate.getTime())
  }

  const matches = [...offsets]
    .map((offset) => new Date(wallTime - offset))
    .filter((candidate) => sameParts(readParts(formatter, candidate), parts))
    .sort((a, b) => a.getTime() - b.getTime())

  // Repeated daylight-saving hours map to the earliest matching instant so the
  // interval includes both copies of the displayed bucket.
  return matches[0] ?? new Date(wallTime)
}

export const getTrendPointRange = (
  date: string,
  granularity: 'hour' | 'day',
  timeZone: string,
): TrendPointRange | null => {
  const match = /^(\d{4})-(\d{2})-(\d{2})(?: (\d{2}):00)?$/.exec(date)
  if (!match) return null

  const [, year, month, day, hour] = match
  if (granularity === 'hour' && hour === undefined) return null
  const startWall = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day), granularity === 'hour' ? Number(hour) : 0))
  const endWall = new Date(startWall.getTime() + (granularity === 'hour' ? 60 * 60 * 1000 : 24 * 60 * 60 * 1000))
  const parts = (wall: Date): LocalParts => ({
    year: wall.getUTCFullYear(),
    month: wall.getUTCMonth() + 1,
    day: wall.getUTCDate(),
    hour: wall.getUTCHours(),
    minute: wall.getUTCMinutes(),
    second: 0,
  })

  let formatter: Intl.DateTimeFormat
  try {
    formatter = makeFormatter(timeZone)
  } catch {
    formatter = makeFormatter('UTC')
  }

  const start = toInstant(parts(startWall), formatter)
  const end = toInstant(parts(endWall), formatter)
  return end > start ? { start, end } : null
}

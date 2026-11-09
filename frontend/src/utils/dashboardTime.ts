// Reporting dates are independent of the server's upstream quota reset clock.
export const DASHBOARD_TIMEZONE = 'Asia/Shanghai'

export function dashboardDate(date = new Date(), offsetDays = 0): string {
  const calendar = new Date(date.getTime() + 8 * 60 * 60 * 1000)
  calendar.setUTCDate(calendar.getUTCDate() + offsetDays)
  return calendar.toISOString().slice(0, 10)
}

export function dateInTimeZone(date: Date, timeZone: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(date)
  const value = (type: string) => parts.find(part => part.type === type)?.value
  return `${value('year')}-${value('month')}-${value('day')}`
}

export function dashboardDateTime(value: string): string {
  return new Date(value).toLocaleString('zh-CN', { timeZone: DASHBOARD_TIMEZONE, hour12: false })
}

// datetime-local has no offset. Reporting controls interpret it in Beijing,
// independently of the browser's timezone and daylight saving transitions.
export function reportingDateTimeInput(date: Date): string {
  if (Number.isNaN(date.getTime())) return ''
  return new Date(date.getTime() + 8 * 60 * 60 * 1000).toISOString().slice(0, 16)
}

export function parseReportingDateTimeInput(value: string): Date | null {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) return null
  const date = new Date(`${value}:00+08:00`)
  if (Number.isNaN(date.getTime()) || reportingDateTimeInput(date) !== value) return null
  return date
}


// Stable, unambiguous Beijing timestamp for CSV/XLSX exports.
export function reportingTimestamp(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Date(date.getTime() + 8 * 60 * 60 * 1000).toISOString().replace('Z', '+08:00')
}

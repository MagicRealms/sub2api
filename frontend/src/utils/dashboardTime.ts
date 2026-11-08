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

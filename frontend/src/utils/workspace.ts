/** Keep summaries scannable; exact token counts remain available in details. */
export function formatTokenMillions(value: number): string {
  const tokens = Math.max(0, Number(value) || 0)
  if (!tokens) return '0 M'
  if (tokens < 10_000) return '<0.01 M'
  return `${(tokens / 1_000_000).toLocaleString('en-US', { maximumFractionDigits: 2 })} M`
}

export const workspacePalette = ['#6c9ee5', '#69c4b1', '#a498d9', '#e8be7e', '#ed9eac', '#74b8cf', '#b5c88b', '#c49fc1']

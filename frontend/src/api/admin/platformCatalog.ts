import { apiClient } from '../client'
import type { PlatformCatalog } from '@/constants/platformCatalog'

// Full provider defaults are administrator-only. The public frontend bundle
// intentionally carries a redacted catalog without upstream base URLs.
export async function getPlatformCatalog(): Promise<PlatformCatalog> {
  const { data } = await apiClient.get<PlatformCatalog>('/admin/platform-catalog')
  return data
}

export const platformCatalogAPI = { getPlatformCatalog }
export default platformCatalogAPI

import { BUILTIN_PLATFORM_CATALOG, type PlatformCatalog } from '../platformCatalog'

// This fixture models the authenticated admin endpoint. Production assets must
// never import it; their embedded catalog contains only the public identity.
export const ADMIN_PLATFORM_CATALOG: PlatformCatalog = {
  platforms: BUILTIN_PLATFORM_CATALOG.platforms.map(spec => spec.id === 'route_relay'
    ? {
      ...spec,
      id: 'sharellm',
      display_name: 'ShareLLM 聚合',
      multi_protocol: {
        ...spec.multi_protocol!,
        modes: spec.multi_protocol!.modes.map(mode => ({
          ...mode,
          base_urls: {
            anthropic: 'https://sharellm.net',
            chat_completions: 'https://sharellm.net/v1',
            responses: 'https://sharellm.net/v1'
          }
        }))
      }
    }
    : spec),
  composite_precedence: BUILTIN_PLATFORM_CATALOG.composite_precedence.map(id => id === 'route_relay' ? 'sharellm' : id)
}

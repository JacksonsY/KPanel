export const panelAccessSection = 'panel-access'

const domainPattern = /^(?=.{1,253}$)(?:[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?\.)+[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?$/

/**
 * Settings route that verifies a freshly bound panel domain and offers the
 * jump to it. The App Market sends KPanel itself here instead of blocking
 * IP+port access before the new address is proven to work.
 */
export function panelDomainSwitchPath(domain: string): string {
  return `/settings?section=${panelAccessSection}&domain=${encodeURIComponent(domain)}`
}

export function panelDomainQuery(value: unknown): string {
  if (typeof value !== 'string') return ''
  const domain = value.trim().toLowerCase()
  return domainPattern.test(domain) && !/^\d+(?:\.\d+){3}$/.test(domain) ? domain : ''
}

export interface PanelLocation {
  protocol: string
  host: string
  origin: string
}

export function currentPanelLocation(): PanelLocation {
  const { protocol, host, origin } = window.location
  return { protocol, host, origin }
}

export function navigatePanel(target: string): void {
  window.location.assign(target)
}

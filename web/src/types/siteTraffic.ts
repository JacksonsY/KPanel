export interface TrafficMetric { requests: number; bytes: number; errors: number; uniqueIPs: number }
export interface TrafficPoint extends TrafficMetric { at: string }
export interface TrafficRanking extends TrafficMetric { name: string }
export interface TrafficCountry extends TrafficMetric { code: string }
export interface SecurityEvent { id: string; at: string; host: string; path: string; country: string; action: string; rules: string[]; severity: number }
export interface SiteTrafficSnapshot extends TrafficMetric {
 period: string; from: string; to: string; generatedAt: string; state: string; enabled: boolean; accessSource: boolean
 trend: TrafficPoint[]; countries: TrafficCountry[]; paths: TrafficRanking[]; referrers: TrafficRanking[]; hosts: string[]
 locatedRequests: number; geoStatus: string; startedAt?: string; collectedAt?: string; catchingUp: boolean; siteAttribution: boolean; warnings: string[]
 protection: { events: number; blocked: number; limited: number; matched: number; rules: TrafficRanking[]; recent: SecurityEvent[]; trend: TrafficPoint[]; countries: TrafficCountry[]; sourceAvailable: boolean }
}

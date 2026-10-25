import type { PhraseCatalog } from '@/i18n/phrase'

export default [
  ["选择地区查看详情", "Select a region to explore"],
  ["地图操作", "Map controls"],
  ["缩小地图", "Zoom out"],
  ["放大地图", "Zoom in"],
  ["恢复全球视角", "Reset world view"],
  ["暂无地区数据", "No attributed region data"],
  ["放大后可拖动地图", "Zoom in to pan the map"],
  ["键盘：加减缩放，方向键平移，0 复位", "Keyboard: +/− zoom, arrows pan, 0 resets"],
  ["已选地区", "Selected region"],
  ["占比", "share"],
  ["定位所选地区", "Focus selected region"],
  ["此地图未收录该地区边界，统计仍已计入。", "This map does not include this region’s boundary. Its statistics are still included."],
  ["点击地图或右侧排行，查看地区请求与占比。", "Select the map or region ranking to view its count and share."],
  ["源站日志统计", "Origin log statistics"],
  ["个地区", "regions"],
  [
    "尚未启用",
    "Not enabled"
  ],
  [
    "开始采集",
    "Starting collection"
  ],
  [
    "持续采集",
    "Collecting"
  ],
  [
    "等待日志",
    "Waiting for logs"
  ],
  [
    "采集暂不可用",
    "Collection unavailable"
  ],
  [
    "正在处理积压",
    "Processing backlog"
  ],
  [
    "状态待核验",
    "Status unavailable"
  ],
  [
    "首次采集从最近日志开始，早期日志不计入历史。",
    "Initial collection starts with recent logs. Earlier entries are excluded."
  ],
  [
    "日志发生截断，部分历史请求可能缺失。",
    "Logs were truncated. Some historical requests may be missing."
  ],
  [
    "日志来源超过采集上限，统计覆盖不完整。",
    "Too many log sources. Statistics may be incomplete."
  ],
  [
    "日志目录条目较多，统计覆盖不完整。",
    "The log directory has too many entries. Coverage may be incomplete."
  ],
  [
    "部分日志无法读取，其他来源仍在采集。",
    "Some logs are unreadable. Collection continues from other sources."
  ],
  [
    "部分日志格式无法识别，已跳过。",
    "Unrecognized log entries were skipped."
  ],
  [
    "统计库已达容量阈值，最旧统计已清理。",
    "The storage threshold was reached. The oldest statistics were removed."
  ],
  [
    "共享访问日志没有站点字段，无法归属的请求仅计入全站总览。",
    "Shared access logs lack a site field. Unattributed requests appear only in the overall view."
  ],
  [
    "采集暂时不可用，已保存的历史数据仍可查看。",
    "Collection is temporarily unavailable. Saved history remains available."
  ],
  [
    "部分数据暂不可用",
    "Some data is unavailable"
  ],
  [
    "未知地区",
    "Unknown region"
  ],
  [
    "确认拦截",
    "Confirmed blocks"
  ],
  [
    "限流拒绝",
    "Rate limit rejections"
  ],
  [
    "规则命中",
    "Rule matches"
  ],
  [
    "检测记录",
    "Detection records"
  ],
  [
    "延迟处理",
    "Delayed requests"
  ],
  [
    "待核验",
    "Unverified"
  ],
  [
    "请求频率限制",
    "Request rate limit"
  ],
  [
    "并发连接限制",
    "Connection limit"
  ],
  [
    "监控数据读取失败",
    "Unable to load monitoring data"
  ],
  [
    "监控已开启，正在采集网站日志。",
    "Monitoring enabled. Collecting website logs."
  ],
  [
    "采集已暂停，历史统计保留。",
    "Collection paused. History is retained."
  ],
  [
    "历史统计已清空，原始日志保持不变。",
    "Monitoring history cleared. Original logs are preserved."
  ],
  [
    "监控操作失败",
    "Monitoring action failed"
  ],
  [
    "网站监控",
    "Website monitoring"
  ],
  [
    "流量与安全，一目了然",
    "Your traffic and security at a glance"
  ],
  [
    "从访问来源到防御事件，持续了解您的网站。",
    "Understand your websites through traffic sources and security events."
  ],
  [
    "网站监控暂不可用",
    "Website monitoring unavailable"
  ],
  [
    "监控视图",
    "Monitoring view"
  ],
  [
    "站点筛选",
    "Filter by site"
  ],
  [
    "全部站点",
    "All sites"
  ],
  [
    "统计时间",
    "Time range"
  ],
  [
    "近 24 小时",
    "Last 24 hours"
  ],
  [
    "近 7 天",
    "Last 7 days"
  ],
  [
    "近 30 天",
    "Last 30 days"
  ],
  [
    "统计口径与设置",
    "Metrics and settings"
  ],
  [
    "采集已暂停",
    "Collection paused"
  ],
  [
    "开启您的网站监控",
    "Start monitoring your websites"
  ],
  [
    "增量采集访问与防御日志，保留最长 30 天的聚合统计。",
    "Collect access and security logs incrementally. Keep up to 30 days of aggregated history."
  ],
  [
    "尚未发现可采集的网站日志。请检查站点日志是否已启用。",
    "No website logs found. Check that logging is enabled for your sites."
  ],
  [
    "请求数",
    "Requests"
  ],
  [
    "源站收到的访问请求",
    "Requests received by the origin"
  ],
  [
    "独立 IP",
    "Unique IPs"
  ],
  [
    "所选时间内按 IP 去重",
    "Distinct IPs in the selected period"
  ],
  [
    "响应流量",
    "Response traffic"
  ],
  [
    "HTTP 响应正文大小",
    "HTTP response body bytes"
  ],
  [
    "HTTP 错误率",
    "HTTP error rate"
  ],
  [
    "4xx 与 5xx 请求占比",
    "Share of 4xx and 5xx requests"
  ],
  [
    "安全事件",
    "Security events"
  ],
  [
    "同一事务合并记录",
    "Records merged by transaction"
  ],
  [
    "来自明确的拦截记录",
    "Based on explicit blocking records"
  ],
  [
    "请求频率或连接数限制",
    "Request rate or connection limits"
  ],
  [
    "其他防御事件",
    "Other security events"
  ],
  [
    "命中、检测与延迟处理",
    "Matches, detections and delays"
  ],
  [
    "访问趋势",
    "Traffic trends"
  ],
  [
    "防御事件趋势",
    "Security event trends"
  ],
  [
    "请求变化与访问高峰",
    "Request volume and traffic peaks"
  ],
  [
    "规则命中与防御活动",
    "Rule matches and defense activity"
  ],
  [
    "访问请求趋势图",
    "Traffic request trend chart"
  ],
  [
    "防御事件趋势图",
    "Security event trend chart"
  ],
  [
    "暂无防御事件",
    "No security events yet"
  ],
  [
    "等待第一批访问数据",
    "Waiting for traffic data"
  ],
  [
    "事件到达后将在这里显示趋势。",
    "Trends will appear when events arrive."
  ],
  [
    "开启监控后，收到的网站请求会显示在这里。",
    "Enable monitoring to see incoming website requests here."
  ],
  [
    "世界访问来源",
    "Traffic around the world"
  ],
  [
    "安全事件来源",
    "Security event origins"
  ],
  [
    "按 IP 归属地区统计",
    "Regions estimated from IP addresses"
  ],
  [
    "世界来源分布地图",
    "World origin distribution map"
  ],
  [
    "请求",
    "requests"
  ],
  [
    "事件",
    "events"
  ],
  [
    "地区数据库无法读取，请更换有效的国家数据库。",
    "The location database is unreadable. Replace it with a valid country database."
  ],
  [
    "地区识别尚未配置，访问与防御统计仍会继续采集。",
    "Location lookup is not configured. Traffic and security collection continues."
  ],
  [
    "无数据",
    "No data"
  ],
  [
    "更多",
    "More"
  ],
  [
    "国家与地区",
    "Countries and regions"
  ],
  [
    "按请求数排序",
    "Ranked by requests"
  ],
  [
    "按事件数排序",
    "Ranked by events"
  ],
  [
    "暂无可识别的地区数据",
    "No identified regions yet"
  ],
  [
    "热门路径",
    "Top paths"
  ],
  [
    "已移除 URL 查询参数",
    "URL query parameters removed"
  ],
  [
    "暂无路径统计",
    "No path statistics yet"
  ],
  [
    "访问来源",
    "Referrers"
  ],
  [
    "请求中记录的来源域名",
    "Referrer domains recorded in requests"
  ],
  [
    "直接访问或无来源",
    "Direct or no referrer"
  ],
  [
    "暂无来源统计",
    "No referrer statistics yet"
  ],
  [
    "尚未发现 WAF 事件日志",
    "No WAF event logs found"
  ],
  [
    "启用兼容的 WAF 日志后，防御事件会自动汇入。",
    "Security events appear automatically when compatible WAF logging is enabled."
  ],
  [
    "命中规则排行",
    "Top matching rules"
  ],
  [
    "一次请求可能命中多条规则",
    "A request may match multiple rules"
  ],
  [
    "暂无规则命中记录",
    "No rule matches yet"
  ],
  [
    "最近安全事件",
    "Recent security events"
  ],
  [
    "最近 50 条，保留最长 7 天",
    "Latest 50 events, retained up to 7 days"
  ],
  [
    "时间",
    "Time"
  ],
  [
    "站点与路径",
    "Site and path"
  ],
  [
    "处理结果",
    "Outcome"
  ],
  [
    "命中规则",
    "Matched rules"
  ],
  [
    "共享日志",
    "Shared log"
  ],
  [
    "暂无安全事件记录",
    "No security events yet"
  ],
  [
    "采集与存储",
    "Collection and storage"
  ],
  [
    "每 10 秒增量读取可用日志，统计与读取进度一并提交。聚合历史最长保留 30 天，安全事件最长保留 7 天或 5000 条。统计库上限 128 MiB，接近容量时清理最旧统计。",
    "Read available logs incrementally every 10 seconds. Statistics and read positions are committed together. Aggregates are retained up to 30 days; recent security events up to 7 days or 5,000 entries. Storage is capped at 128 MiB; the oldest aggregates are removed near capacity."
  ],
  [
    "流量统计口径",
    "Traffic metric definitions"
  ],
  [
    "请求数包含页面、静态资源与机器人请求；独立 IP 不等同于独立访客。响应流量为 HTTP 正文大小，不等同于服务器账单流量。CDN 直接命中的访问不会到达源站。",
    "Requests include pages, static assets and bots. Unique IPs are not unique visitors. Response traffic counts HTTP body bytes, not billed server bandwidth. Requests served directly by a CDN do not reach the origin."
  ],
  [
    "本地地区识别",
    "Local location lookup"
  ],
  [
    "使用本地 MMDB 国家数据库。通过文件管理将国家库上传到下面的位置，后续请求会自动识别地区。",
    "Use a local MMDB country database. Upload a country database to this location in File Manager to identify regions for subsequent requests."
  ],
  [
    "地区为 IP 归属推测。CDN 或代理部署须先正确恢复真实访客 IP；统计不会直接信任请求中的转发头。",
    "Regions are estimated from IP addresses. CDN and proxy deployments must restore real client IPs correctly. Collection does not directly trust forwarded request headers."
  ],
  [
    "防御事件口径",
    "Security event definitions"
  ],
  [
    "审计日志记录规则命中，明确的拦截日志才计为确认拦截；普通 403 不会直接计为 WAF 拦截。原始请求头、请求体及规则匹配内容不写入统计库。",
    "Audit logs indicate rule matches. Only explicit blocking records count as confirmed blocks; ordinary 403 responses do not. Raw headers, request bodies and matched payloads are excluded from statistics."
  ],
  [
    "将清除已保存的流量与防御历史。原始 Nginx 日志和 WAF 配置保持不变，采集进度会保留。",
    "Clear saved traffic and security history. Original Nginx logs, WAF settings and collection checkpoints are preserved."
  ],
  [
    "清空网站监控统计",
    "Clear website monitoring history"
  ],
  [
    "流量分析",
    "Traffic analytics"
  ],
  [
    "WAF 防御",
    "WAF defense"
  ],
  [
    "历史统计",
    "Saved history"
  ],
  [
    "暂停采集",
    "Pause collection"
  ],
  [
    "清空统计",
    "Clear statistics"
  ],
  [
    "开始监控",
    "Start monitoring"
  ],
  [
    "配置地区识别",
    "Set up location lookup"
  ],
  [
    "获取国家数据库",
    "Get a country database"
  ],
  [
    "打开文件管理",
    "Open File Manager"
  ],
  [
    "查看原始日志",
    "View original logs"
  ],
  [
    "管理 WAF",
    "Manage WAF"
  ],
  [
    "确认清空",
    "Confirm clear"
  ],
  [
    "取消选择",
    "Clear selection"
  ],
  [
    "数据覆盖说明",
    "Data coverage"
  ]
] satisfies PhraseCatalog

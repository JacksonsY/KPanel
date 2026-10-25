// Synthetic loopback preview data; never imported by production code.
let enabled = true, cleared = false
export async function mockSiteMonitoring(request, response, url, send, readJSON) {
 if (url.pathname === '/api/v1/site-traffic/actions' && request.method === 'POST') {
  const { action } = await readJSON(request)
  if (action === 'clear') cleared = true
  else if (action === 'enable' || action === 'pause') enabled = action === 'enable'
  else { send(response,422,{code:'invalid_traffic_action'});return true }
  send(response,200,{ok:true}); return true
 }
 if (url.pathname !== '/api/v1/site-traffic' || request.method !== 'GET') return false
 const now = Date.now(), period = url.searchParams.get('period') || '24h'
 const hours = period === '30d' ? 720 : period === '7d' ? 168 : 24
 const host = url.searchParams.get('host') || 'example.com'
 const trend = Array.from({length:24},(_,i)=>({at:new Date(now-(23-i)*hours/24*3600000).toISOString(),requests:cleared?0:Math.round(160+120*Math.sin(i/2)**2),bytes:0,errors:0,uniqueIPs:0}))
 const requests = trend.reduce((sum,p)=>sum+p.requests,0), countries = [['US',.32],['CN',.23],['DE',.16],['JP',.12],['SG',.08],['GB',.05],['ZZ',.04]].map(([code,share])=>({code,requests:Math.round(requests*share),bytes:0,errors:0,uniqueIPs:0}))
 const securityTrend = trend.map(p=>({...p,requests:Math.round(p.requests/18)})), events=securityTrend.reduce((sum,p)=>sum+p.requests,0)
 const matched = cleared?0:32, limited=cleared?0:55
 send(response,200,{enabled,accessSource:true,period,from:new Date(now-hours*3600000).toISOString(),to:new Date(now).toISOString(),generatedAt:new Date(now).toISOString(),state:enabled?'active':'disabled',requests,bytes:requests*14025,errors:Math.round(requests*.013),uniqueIPs:Math.round(requests*.31),trend,countries,paths:cleared?[]:[{name:'/',requests:2240},{name:'/blog',requests:1253},{name:'/api/articles',requests:763},{name:'/assets/app.js',requests:540}],referrers:cleared?[]:[{name:'',requests:2430},{name:'www.google.com',requests:1380},{name:'github.com',requests:560}],hosts:['example.com','blog.example.com'],locatedRequests:Math.round(requests*.96),geoStatus:'ready',startedAt:new Date(now-720*3600000).toISOString(),collectedAt:new Date(now).toISOString(),catchingUp:false,siteAttribution:true,warnings:[],protection:{events,blocked:events-limited-matched,limited,matched,sourceAvailable:true,trend:securityTrend,countries:countries.map(c=>({...c,requests:Math.round(c.requests/18)})),rules:cleared?[]:[{name:'942100',requests:94},{name:'949110',requests:81},{name:'nginx.limit_req',requests:55},{name:'920350',requests:23}],recent:cleared?[]:Array.from({length:7},(_,i)=>({id:'mock-event-'+i,at:new Date(now-i*73000).toISOString(),host,path:['/wp-login.php','/api/search','/admin'][i%3],country:['US','CN','DE'][i%3],action:['blocked','limited','matched'][i%3],rules:[['942100','949110'],['nginx.limit_req'],['920350']][i%3],severity:2}))}})
 return true
}

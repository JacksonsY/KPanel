// UI preview fixtures only; production video discovery lives in tutorial_feed.go.
export function mockTutorials(request, response, url, send) {
  if (url.pathname !== '/api/v1/tutorials/videos') return false
  const youtube = url.searchParams.get('platform') === 'youtube'
  const ids = youtube
    ? ['4bdTRvOmIXc', '6OFR8TxFguo', '3HwmSTCtozg', 'kNBMTGF4G1k', 'HLTtsALHlpw', '9z2webQ89pM', 'qWs4xjed_yM', 'Azlahc0Loo0', 'a_yATf3ocmQ']
    : ['BV1rvpe6dE9k', 'BV13xpb6tEq7', 'BV1J2HX6ME7p', 'BV14UH66EEZ9', 'BV1bLHv6vEAi', 'BV1e7ar69EfD', 'BV12haz6TE9E', 'BV1YKaY6FEzk', 'BV1jNe46WEjf']
  send(response, 200, {
    stale: false,
    videos: ids.map((id, index) => ({
      url: youtube ? `https://www.youtube.com/watch?v=${id}&hl=en` : `https://www.bilibili.com/video/${id}/`,
      title: `${youtube ? 'KPanel tutorial preview' : 'KPanel 教程预览'} ${index + 1}`,
      thumbnailUrl: '/tutorials/kpanel-video-placeholder.svg',
    })),
  })
  return true
}

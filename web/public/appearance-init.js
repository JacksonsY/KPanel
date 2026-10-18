// Run before the application modules, under the existing script-src 'self' policy.
(() => {
  const root = document.documentElement
  const read = key => {
    try { return localStorage.getItem(key) } catch { return null }
  }
  const preference = read('kejilion-panel-theme')
  const systemTheme = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  const theme = preference === 'light' || preference === 'dark'
    ? preference : systemTheme
  root.dataset.theme = theme
  root.style.colorScheme = theme
  const readSession = key => {
    try { return sessionStorage.getItem(key) } catch { return null }
  }
  // Restore only locally generated gradient/color tokens, never CSS URLs from storage.
  try {
    const raw = readSession('kpanel:desktop-backdrop:v1')
    const cached = raw && raw.length < 12000 ? JSON.parse(raw) : null
    if (cached?.theme === theme && cached.colors === read('kejilion-panel-colors')) {
      for (const token of ['--bg', '--brand-soft', '--desktop-wallpaper-base', '--desktop-wallpaper-veil-light', '--desktop-wallpaper-veil-dark', '--desktop-wallpaper-vignette', '--desktop-aurora-one', '--desktop-aurora-two', '--desktop-aurora-opacity']) {
        const value = cached.tokens?.[token]
        if (typeof value === 'string' && value.length < 1500 && /^[a-zA-Z0-9#.,()% /+-]+$/.test(value) && !/url/i.test(value)) root.style.setProperty(token, value)
      }
    }
  } catch { /* Invalid or unavailable cache leaves the shared theme defaults. */ }
  // Private and desktop assets still wait for the authenticated appearance snapshot.
  root.style.setProperty('--desktop-wallpaper-image', 'none')
  root.style.setProperty('--auth-wallpaper-image', 'none')
  const publicPage = /^\/(login|setup|share)(\/|$)/.test(location.pathname)
  const builtinURL = id => {
    if (id === 'xp') return '/wallpapers/kpanel-desktop-xp.svg'
    return ['classic', 'orbit', 'horizon', 'rift', 'prism'].includes(id)
      ? `/wallpapers/kpanel-desktop${id === 'classic' ? '' : `-${id}`}.webp` : null
  }
  const cacheKey = id => `kpanel:desktop-wallpaper-cache:v1:${id}`
  const cachedImage = id => {
    const value = readSession(cacheKey(id))
    return value && value.length <= 131072 && /^data:image\/webp;base64,[A-Za-z0-9+/]+=*$/.test(value) ? value : null
  }
  const caching = new Set()
  const cacheWallpaper = async id => {
    const url = builtinURL(id)
    if (!url || cachedImage(id) || caching.has(id)) return
    caching.add(id)
    try {
      const response = await fetch(url, { cache: 'force-cache' })
      if (!response.ok) return
      const blob = await response.blob()
      if (blob.type !== 'image/webp' || blob.size > 98304) return
      await new Promise(resolve => {
        const reader = new FileReader()
        reader.onload = () => {
          try { sessionStorage.setItem(cacheKey(id), reader.result) } catch { /* Optional paint cache. */ }
        }
        reader.onloadend = resolve
        reader.readAsDataURL(blob)
      })
    } catch { /* The authenticated URL remains usable without a cache. */ }
    finally { caching.delete(id) }
  }
  // This is a public built-in paint hint, never a settings write or a private asset preview.
  const id = read('kpanel:desktop-wallpaper:v1')
  const level = read('kpanel:classic-wallpaper:v1')
  const url = builtinURL(id)
  if (!publicPage && read('kejilion-panel-desktop-mode') !== 'desktop'
    && (level === 'ambient' || level === 'clear') && url) {
    const cached = cachedImage(id)
    root.dataset.classicWallpaper = level
    root.style.setProperty('--classic-wallpaper-image', `url("${cached || url}")`)
    if (cached) root.classList.add('classic-wallpaper-boot')
    const image = new Image()
    image.fetchPriority = 'high'
    image.src = cached || url
    void image.decode().catch(() => {
      try { sessionStorage.removeItem(cacheKey(id)) } catch { /* Optional paint cache. */ }
      root.classList.remove('classic-wallpaper-boot')
      if (read('kpanel:desktop-wallpaper:v1') === id) root.style.setProperty('--classic-wallpaper-image', `url("${url}")`)
    })
  }
  window.addEventListener('kpanel:cache-classic-wallpaper', event => { void cacheWallpaper(event.detail?.id) })
  if (read('kejilion-panel-desktop-mode') === 'desktop' && !publicPage) {
    root.classList.add('desktop-boot')
  }
})()

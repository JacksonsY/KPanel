import { useI18n } from '@/i18n'

const zh = {
  tabs: '标签页', more: '更多', done: '完成',
  title: '浏览器', address: '输入网址或搜索关键词', go: '访问', newTab: '新建标签页', closeTab: '关闭标签页',
  back: '后退', forward: '前进', reload: '刷新', stop: '停止加载', home: '起始页', external: '用系统浏览器打开',
  server: '通过服务器访问', connecting: '正在连接浏览器…', connected: '已连接', disconnected: '连接已断开', retry: '重新连接',
  startTitle: '在工作台里打开网页', startHint: '搜索资料，或访问服务器上的应用。网页通过服务器网络打开。', quick: '应用与网站',
  unconfigured: '浏览器组件尚未启用', unconfiguredHint: '此功能需要独立的浏览器组件。启用后重新连接，也可以用系统浏览器打开链接。',
  unavailable: '暂时无法连接浏览器服务', unavailableHint: '请检查浏览器组件是否正在运行，然后重新连接。',
  invalid: '请输入有效的 HTTP / HTTPS 网址或搜索关键词。', loading: '正在打开网页…', untitled: '新标签页',
  limit: '最多同时打开 8 个标签页，请先关闭一个。', failed: '操作未完成，请重试或用系统浏览器打开。',
  startFailed: '浏览器启动失败，请检查组件中的 Chromium 和运行权限。', upload: '当前版本尚未接入文件上传，请用系统浏览器完成。',
  dialog: '网页提示', accept: '确定', cancel: '取消', paused: '画面已暂停', screen: '远程网页，可使用鼠标和键盘操作',
  input: '向网页输入文字', send: '输入', clear: '关闭提示', sessionHint: '短暂断线可重新连接；浏览器组件停止后，网页登录状态不会保留。',
}
const en: Record<keyof typeof zh, string> = {
  tabs: 'Tabs', more: 'More', done: 'Done',
  title: 'Browser', address: 'Enter a URL or search', go: 'Go', newTab: 'New tab', closeTab: 'Close tab', back: 'Back', forward: 'Forward', reload: 'Reload', stop: 'Stop loading', home: 'Start page', external: 'Open in system browser', server: 'Via server network', connecting: 'Connecting to browser…', connected: 'Connected', disconnected: 'Disconnected', retry: 'Reconnect', startTitle: 'Browse in your workspace', startHint: 'Find information or open your applications. Pages use the server network.', quick: 'Apps and websites', unconfigured: 'Browser component is not enabled', unconfiguredHint: 'This feature needs a separate browser component. Enable it and reconnect, or open the link in your system browser.', unavailable: 'Browser service is unavailable', unavailableHint: 'Check that the browser component is running, then reconnect.', invalid: 'Enter a valid HTTP / HTTPS URL or search query.', loading: 'Opening page…', untitled: 'New tab', limit: 'Up to 8 tabs can be open. Close one first.', failed: 'The action failed. Retry or open in your system browser.', startFailed: 'The browser could not start. Check Chromium and the component permissions.', upload: 'File upload is not connected yet. Use your system browser.', dialog: 'Website message', accept: 'OK', cancel: 'Cancel', paused: 'Screen paused', screen: 'Remote webpage, controlled by mouse and keyboard', input: 'Enter text on the webpage', send: 'Insert', clear: 'Dismiss notice', sessionHint: 'Reconnect after a short interruption. Website sessions are not retained after the browser component stops.',
}
const tw: Record<keyof typeof zh, string> = {
  ...zh, tabs: '分頁', more: '更多', done: '完成', title: '瀏覽器', address: '輸入網址或搜尋關鍵字', go: '前往', newTab: '新增分頁', closeTab: '關閉分頁', back: '上一頁', forward: '下一頁', reload: '重新整理', stop: '停止載入', home: '起始頁', external: '以系統瀏覽器開啟', server: '透過伺服器存取', connecting: '正在連線瀏覽器…', connected: '已連線', disconnected: '連線已中斷', retry: '重新連線', startTitle: '在工作台裡開啟網頁', startHint: '搜尋資料，或存取伺服器上的應用。網頁透過伺服器網路開啟。', quick: '應用與網站', unconfigured: '瀏覽器元件尚未啟用', unconfiguredHint: '此功能需要獨立的瀏覽器元件。啟用後重新連線，也可以使用系統瀏覽器開啟連結。', unavailable: '暫時無法連線瀏覽器服務', unavailableHint: '請檢查瀏覽器元件是否正在執行，然後重新連線。', invalid: '請輸入有效的 HTTP / HTTPS 網址或搜尋關鍵字。', loading: '正在開啟網頁…', untitled: '新分頁', limit: '最多同時開啟 8 個分頁，請先關閉一個。', failed: '操作未完成，請重試或使用系統瀏覽器開啟。', startFailed: '瀏覽器啟動失敗，請檢查元件中的 Chromium 與執行權限。', upload: '目前版本尚未接入檔案上傳，請使用系統瀏覽器完成。', dialog: '網頁提示', accept: '確定', cancel: '取消', paused: '畫面已暫停', screen: '遠端網頁，可使用滑鼠和鍵盤操作', input: '向網頁輸入文字', send: '輸入', clear: '關閉提示', sessionHint: '短暫斷線可重新連線；瀏覽器元件停止後，網頁登入狀態不會保留。',
}
export function useBrowserText() { const { locale } = useI18n(); return (key: keyof typeof zh) => (locale.value === 'en-US' ? en : locale.value === 'zh-TW' ? tw : zh)[key] }

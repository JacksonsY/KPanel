import type { DesktopShortcut } from '@/types/api'

/** Remote commands have a distinct type so cached local-only clients cannot execute them. */
export function isDesktopCommandShortcut(shortcut?: Pick<DesktopShortcut, 'targetType'>): boolean {
  return shortcut?.targetType === 'command' || shortcut?.targetType === 'remote-command'
}

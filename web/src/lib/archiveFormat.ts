export function archiveFormatForName(name: string): 'tar.gz' | 'zip' | 'tar' | undefined {
  const lower = name.toLowerCase()
  if (lower.endsWith('.tar.gz') || lower.endsWith('.tgz')) return 'tar.gz'
  if (lower.endsWith('.zip')) return 'zip'
  if (lower.endsWith('.tar')) return 'tar'
  return undefined
}

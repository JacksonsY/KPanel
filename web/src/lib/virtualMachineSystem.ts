// Match libosinfo identities, never VM names, disk names or arbitrary substrings.
const systems: Record<string, { distro: string; label: string }> = {
  'debian.org/debian': { distro: 'debian', label: 'Debian' },
  'ubuntu.com/ubuntu': { distro: 'ubuntu', label: 'Ubuntu' },
  'centos.org/centos': { distro: 'centos', label: 'CentOS' },
  'rockylinux.org/rocky': { distro: 'rocky', label: 'Rocky Linux' },
  'almalinux.org/almalinux': { distro: 'alma', label: 'AlmaLinux' },
  'fedoraproject.org/fedora': { distro: 'fedora', label: 'Fedora' },
  'alpinelinux.org/alpinelinux': { distro: 'alpine', label: 'Alpine Linux' },
  'archlinux.org/archlinux': { distro: 'arch', label: 'Arch Linux' },
  'manjaro.org/manjaro': { distro: 'manjaro', label: 'Manjaro' },
  'redhat.com/rhel': { distro: 'rhel', label: 'Red Hat Enterprise Linux' },
  'opensuse.org/opensuse': { distro: 'opensuse', label: 'openSUSE' },
  'suse.com/sles': { distro: 'suse', label: 'SUSE Linux Enterprise' },
  'oracle.com/ol': { distro: 'oracle', label: 'Oracle Linux' },
  'microsoft.com/win': { distro: 'windows', label: 'Windows' },
}

export function virtualMachineSystem(osId?: string) {
  if (!osId || osId.length > 256) return undefined
  try {
    const url = new URL(osId)
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.port || url.search || url.hash) return undefined
    const [, family, version] = url.pathname.split('/')
    if (!family || !version) return undefined
    return systems[`${url.hostname}/${family}`]
  } catch { return undefined }
}

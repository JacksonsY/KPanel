import { describe, expect, it } from 'vitest'
import { virtualMachineSystem } from './virtualMachineSystem'

describe('declared virtual machine systems', () => {
  it.each([
    ['http://debian.org/debian/12', 'debian'],
    ['http://ubuntu.com/ubuntu/24.04', 'ubuntu'],
    ['http://microsoft.com/win/11', 'windows'],
    ['http://rockylinux.org/rocky/9.5', 'rocky'],
    ['http://almalinux.org/almalinux/9', 'alma'],
    ['http://opensuse.org/opensuse/tumbleweed', 'opensuse'],
    ['http://alpinelinux.org/alpinelinux/3.20', 'alpine'],
  ])('maps %s to its existing icon', (id, distro) => {
    expect(virtualMachineSystem(id)?.distro).toBe(distro)
  })

  it.each([undefined, '', 'debian-12', 'ubuntu-server', 'http://debian.org.evil/debian/12',
    'http://evil/debian.org/debian/12', 'http://debian.org/unknown/12', 'http://debian.org/debian',
    'http://debian.org/debian/12?ubuntu', 'file://debian.org/debian/12', '<svg onload=alert(1)>',
  ])('keeps unknown declarations generic: %s', id => {
    expect(virtualMachineSystem(id)).toBeUndefined()
  })
})

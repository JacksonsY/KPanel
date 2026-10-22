// Small, valid ZIP fixtures for the explicitly simulated local preview.
import { crc32 } from 'node:zlib'

export const mockArchiveFiles = new Map([
  ['assets/app.js', 'console.log("KPanel mock preview");\n'],
  ['assets/main.css', 'body { margin: 0; }\n'],
  ['index.html', '<!doctype html><title>KPanel mock preview</title>\n'],
  ['README.md', '# KPanel mock archive\nThis file is simulated preview data.\n'],
])

export function mockArchiveZIP(files = mockArchiveFiles) {
  const local = [], central = []
  let offset = 0, directorySize = 0
  for (const [name, content] of files) {
    const filename = Buffer.from(name), data = Buffer.from(content)
    const header = Buffer.alloc(30), record = Buffer.alloc(46)
    header.writeUInt32LE(0x04034b50); header.writeUInt16LE(20, 4)
    header.writeUInt16LE(0x800, 6); header.writeUInt32LE(crc32(data), 14)
    header.writeUInt32LE(data.length, 18); header.writeUInt32LE(data.length, 22)
    header.writeUInt16LE(filename.length, 26)
    record.writeUInt32LE(0x02014b50); record.writeUInt16LE(20, 4); record.writeUInt16LE(20, 6)
    record.writeUInt16LE(0x800, 8); record.writeUInt32LE(crc32(data), 16)
    record.writeUInt32LE(data.length, 20); record.writeUInt32LE(data.length, 24)
    record.writeUInt16LE(filename.length, 28); record.writeUInt32LE(offset, 42)
    local.push(header, filename, data); central.push(record, filename)
    offset += header.length + filename.length + data.length
    directorySize += record.length + filename.length
  }
  const end = Buffer.alloc(22)
  end.writeUInt32LE(0x06054b50); end.writeUInt16LE(files.size, 8); end.writeUInt16LE(files.size, 10)
  end.writeUInt32LE(directorySize, 12); end.writeUInt32LE(offset, 16)
  return Buffer.concat([...local, ...central, end])
}

import { readFileSync } from 'node:fs'
import { parse } from '@vue/compiler-sfc'
import ts from 'typescript'
import { describe, expect, it } from 'vitest'

const navigation = readFileSync(new URL('../lib/navigation.ts', import.meta.url), 'utf8')
const syntax = ts.createSourceFile('navigation.ts', navigation, ts.ScriptTarget.Latest, true)
const pages: { path: string; file: string }[] = []
function collectPages(node: ts.Node): void {
  if (ts.isPropertyAssignment(node) && ts.isStringLiteral(node.name) && node.name.text.startsWith('/')) {
    if (node.name.text === '/setup' || node.name.text === '/login') return
    let file: string | undefined
    function findImport(child: ts.Node): void {
      if (ts.isCallExpression(child) && child.expression.kind === ts.SyntaxKind.ImportKeyword) {
        const target = child.arguments[0]
        if (target && ts.isStringLiteral(target) && target.text.startsWith('@/views/')) file = target.text.slice(8)
      }
      ts.forEachChild(child, findImport)
    }
    findImport(node.initializer)
    if (!file) throw new Error(`unregistered page loader: ${node.name.text}`)
    pages.push({ path: node.name.text, file })
  }
  ts.forEachChild(node, collectPages)
}
collectPages(syntax)
pages.push({ path: '/ai', file: 'AiView.vue' })

// These are intentional workspaces with centrally adapted sizing/scrolling,
// not an exemption for arbitrary new page classes.
const workspaces: Record<string, string> = {
  '/files': 'files-page', '/gallery': 'gallery-page', '/terminal': 'terminal-page',
  '/diagnostics': 'diagnostics-page', '/monitoring': 'monitoring-page', '/ai': 'ai-workspace',
  '/browser': 'web-browser',
}

function rootClasses(file: string, seen = new Set<string>()): string[] {
  expect(seen.has(file), `recursive page wrapper: ${file}`).toBe(false)
  seen.add(file)
  const source = readFileSync(new URL(`../${file}`, import.meta.url), 'utf8')
  const root = parse(source).descriptor.template?.ast?.children.find(node => node.type === 1)
  if (!root || root.type !== 1) throw new Error(`missing page root: ${file}`)
  const classes = root.props.find(prop => prop.type === 6 && prop.name === 'class')
  if (classes?.type === 6) return classes.value?.content.split(/\s+/) ?? []
  const child = source.match(new RegExp(`import ${root.tag} from ['"]\\./([^'"]+\\.vue)['"]`))?.[1]
  if (!child) throw new Error(`unregistered page layout: ${file} <${root.tag}>`)
  return rootClasses(file.slice(0, file.lastIndexOf('/') + 1) + child, seen)
}

describe('shared classic and desktop page layout', () => {
  it.each(pages)('$path opts into shared page spacing or its registered workspace', ({ path, file }) => {
    expect(rootClasses(`views/${file}`), file).toContain(workspaces[path] ?? 'page')
  })
  it('keeps the classic file split wrapper distinct from the desktop file workspace', () => {
    const router = readFileSync(new URL('../router.ts', import.meta.url), 'utf8')
    expect(router).toContain("import('@/components/files/FilesSplitWorkspace.vue')")
    expect(rootClasses('components/files/FilesSplitWorkspace.vue')).toContain('files-workspace')
  })
})

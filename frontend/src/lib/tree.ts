export type TreeNode = {
  name: string
  path: string
  isFolder: boolean
  kind?: string
  method?: string
  children?: TreeNode[]
}

/** Keep requests whose name matches (plus their ancestor folders) and
 *  folders whose name matches (with all their contents). */
export function filterTree(node: TreeNode, query: string): TreeNode | null {
  const q = query.trim().toLowerCase()
  if (!q) return node
  const visit = (n: TreeNode): TreeNode | null => {
    if (n.name.toLowerCase().includes(q) && n.path !== '') return n
    if (!n.isFolder) return null
    const kids = (n.children ?? []).map(visit).filter((c): c is TreeNode => c !== null)
    return kids.length ? { ...n, children: kids } : null
  }
  return visit(node)
}

export function folderPaths(node: TreeNode): string[] {
  if (!node.isFolder) return []
  return [node.path, ...(node.children ?? []).flatMap(folderPaths)]
}

export function parentFolder(path: string): string {
  const i = path.lastIndexOf('/')
  return i < 0 ? '' : path.slice(0, i)
}

/** Must match store.Slug in Go. */
export function slug(name: string): string {
  const s = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  return s || 'request'
}

export function renamedPath(path: string, newName: string, isFolder: boolean): string {
  const parent = parentFolder(path)
  const leaf = slug(newName) + (isFolder ? '' : '.yaml')
  return parent ? `${parent}/${leaf}` : leaf
}

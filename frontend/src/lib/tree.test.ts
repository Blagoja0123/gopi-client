import { describe, expect, it } from 'vitest'
import { filterTree, folderPaths, parentFolder, renamedPath, type TreeNode } from './tree'

const tree: TreeNode = {
  name: 'root', path: '', isFolder: true,
  children: [
    { name: 'users', path: 'users', isFolder: true, children: [
      { name: 'Get user', path: 'users/get-user.yaml', isFolder: false, method: 'GET', kind: 'http' },
      { name: 'admin', path: 'users/admin', isFolder: true, children: [] },
    ] },
    { name: 'Health', path: 'health.yaml', isFolder: false, kind: 'grpc' },
  ],
}

describe('filterTree', () => {
  it('keeps matching requests and their folders', () => {
    const out = filterTree(tree, 'get')!
    expect(out.children!.map((c) => c.name)).toEqual(['users'])
    expect(out.children![0].children!.map((c) => c.name)).toEqual(['Get user'])
  })
  it('matches folder names (keeping the whole folder)', () => {
    const out = filterTree(tree, 'admin')!
    expect(out.children![0].children!.map((c) => c.name)).toEqual(['admin'])
  })
  it('returns the tree unchanged for empty query', () => {
    expect(filterTree(tree, '  ')).toBe(tree)
  })
  it('returns null when nothing matches', () => {
    expect(filterTree(tree, 'zzz')).toBeNull()
  })
})

describe('paths', () => {
  it('lists folders depth-first', () => {
    expect(folderPaths(tree)).toEqual(['', 'users', 'users/admin'])
  })
  it('finds the parent folder', () => {
    expect(parentFolder('users/get-user.yaml')).toBe('users')
    expect(parentFolder('health.yaml')).toBe('')
  })
  it('computes rename targets', () => {
    expect(renamedPath('users/get-user.yaml', 'Fetch User', false)).toBe('users/fetch-user.yaml')
    expect(renamedPath('users/admin', 'Super Admins', true)).toBe('users/super-admins')
  })
})

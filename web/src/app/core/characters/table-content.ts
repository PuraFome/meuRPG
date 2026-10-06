/**
 * The table's own content (RN-23): an entry the master wrote for one campaign has a key that
 * ends in "@mesa" (`class:guardiao-do-vale@mesa`). The one place that says so; the screens write
 * "Da mesa" beside such a name, never by colour alone.
 */
export function isTableKey(key: string): boolean {
  return key.endsWith('@mesa');
}

/** A value with its no-break spaces (the app ties numbers to their units with them) turned into plain ones, so a spec
 * can compare the words it reads. */
export function plainText<T>(value: T): T {
  return JSON.parse(JSON.stringify(value).replace(/ /g, ' ')) as T;
}

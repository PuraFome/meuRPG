/**
 * Specs of the campaign page's phone layout: makes the browser say that a `max-width` media
 * query matches (a phone), until `endPhone()`. The panels read it through `mediaQuery(PHONE_QUERY)`.
 */
export function startPhone(): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    value: (query: string) =>
      ({
        matches: query.includes('max-width'),
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
      }) as unknown as MediaQueryList,
  });
}

export function endPhone(): void {
  Reflect.deleteProperty(window, 'matchMedia');
}

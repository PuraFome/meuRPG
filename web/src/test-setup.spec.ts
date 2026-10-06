// The global test setup (src/test-setup.ts) restores every stubbed global after each test: the spec files are not isolated
// from each other, so a stub left behind would change how a later file behaves. The tests below run in order.
describe('the global test setup', () => {
  const original = window.matchMedia;

  it('lets a test stub a global', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }));
    expect(window.matchMedia).not.toBe(original);
  });

  it('gives the next test the original back', () => {
    expect(window.matchMedia).toBe(original);
  });
});

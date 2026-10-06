// The global test setup (src/test-setup.ts) restores every stubbed global after each test and gives every test a fresh
// scrollIntoView: the spec files are not isolated from each other, so a stub left behind would change how a later file
// behaves. The tests below run in order.
describe('the global test setup', () => {
  const original = window.matchMedia;

  it('lets a test stub a global', () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }));
    expect(window.matchMedia).not.toBe(original);
  });

  it('gives the next test the original back', () => {
    expect(window.matchMedia).toBe(original);
  });

  it('gives every test a scrollIntoView, which jsdom lacks and components call', () => {
    expect(() => document.createElement('div').scrollIntoView()).not.toThrow();
  });

  it('lets a test change scrollIntoView', () => {
    Element.prototype.scrollIntoView = () => {
      throw new Error('changed');
    };
    expect(() => document.createElement('div').scrollIntoView()).toThrow('changed');
  });

  it('gives the next test a fresh one', () => {
    expect(vi.mocked(Element.prototype.scrollIntoView).mock.calls).toHaveLength(0);
    expect(() => document.createElement('div').scrollIntoView()).not.toThrow();
  });
});

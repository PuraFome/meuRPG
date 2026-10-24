import { CAMPAIGN_DETAIL_ROUTES } from './campaign-detail.routes';
import { MesaTab } from './tabs/mesa-tab';
import { PrepTab } from './tabs/prep-tab';
import { SetupTab } from './tabs/setup-tab';

describe('the campaign page routes', () => {
  const page = CAMPAIGN_DETAIL_ROUTES[0];

  it("draw the master's three tabs as children of the page, each lazy: Mesa (the default), prep and setup", async () => {
    const children = page.children ?? [];
    expect(children.map((c) => c.path)).toEqual(['', 'prep', 'setup']);
    expect(children[0].pathMatch).toBe('full');
    const loaded = await Promise.all(
      children.map((c) => (c.loadComponent as () => Promise<unknown>)()),
    );
    expect(loaded).toEqual([MesaTab, PrepTab, SetupTab]);
  });

  it('keep the sources the page and its tabs read, once, on the page route', () => {
    expect(page.path).toBe('');
    expect(page.providers?.length).toBeGreaterThan(0);
    expect(page.loadComponent).toBeDefined();
  });
});

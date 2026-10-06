import { TestBed } from '@angular/core/testing';

import { CONNECT_TRANSPORT } from '../connect/transport';
import { SpellsClient } from './spells-client';

describe('SpellsClient, the caches follow the content version (M8)', () => {
  const asked: string[] = [];
  let version = 'v1';

  function client(): SpellsClient {
    asked.length = 0;
    version = 'v1';
    const transport = {
      unary: async (method: { name: string; output: { fromJson?: unknown } }, _s: unknown, _t: unknown, _h: unknown, input: unknown) => {
        asked.push(method.name);
        const message =
          method.name === 'ListSpells'
            ? { spells: [], nextPageToken: '', total: 0, contentVersion: version }
            : method.name === 'GetSpellDetails'
              ? { spell: { spell: { key: (input as { spellKey: string }).spellKey } } }
              : { content: { classes: [{ key: 'class:wizard', namePt: 'Mago' }], spells: [{ classKeys: ['class:wizard'] }] } };
        return { stream: false, service: {}, method, header: new Headers(), trailer: new Headers(), message };
      },
    };
    TestBed.configureTestingModule({ providers: [{ provide: CONNECT_TRANSPORT, useValue: transport }] });
    return TestBed.inject(SpellsClient);
  }

  it('keeps a spell\'s details and the class names until the version changes, then reads them again', async () => {
    const c = client();
    await c.list({ campaignId: 'camp-1' });
    await c.details('camp-1', 'spell:a');
    await c.details('camp-1', 'spell:a');
    await c.classes('camp-1');
    await c.classes('camp-1');
    expect(asked.filter((n) => n === 'GetSpellDetails')).toHaveLength(1);
    expect(asked.filter((n) => n === 'ListContent')).toHaveLength(1);

    // The master edits the table: the next answer carries another version.
    version = 'v2';
    await c.list({ campaignId: 'camp-1' });
    await c.details('camp-1', 'spell:a');
    await c.classes('camp-1');
    expect(asked.filter((n) => n === 'GetSpellDetails')).toHaveLength(2);
    expect(asked.filter((n) => n === 'ListContent')).toHaveLength(2);

    // The same version again changes nothing.
    await c.list({ campaignId: 'camp-1' });
    await c.details('camp-1', 'spell:a');
    expect(asked.filter((n) => n === 'GetSpellDetails')).toHaveLength(2);
  });
});

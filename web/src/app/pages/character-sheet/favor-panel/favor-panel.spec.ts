import { TestBed } from '@angular/core/testing';

import { FavorClient } from '../../../core/play/favor-client';
import { FakeFavorClient } from '../../../core/play/favor-testing';
import { pensantusVitals } from '../../../core/play/live-testing';
import { FavorPanel } from './favor-panel';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim() ?? '';

function setup(viewer: 'owner' | 'master', holders: readonly string[]) {
  const client = new FakeFavorClient();
  client.holders = holders.map((characterId) => ({ characterId, name: 'Pensantus' }));
  TestBed.configureTestingModule({
    providers: [{ provide: FavorClient, useValue: client.asClient() }],
  });
  const fixture = TestBed.createComponent(FavorPanel);
  fixture.componentRef.setInput('campaignId', 'camp');
  fixture.componentRef.setInput('characterId', 'char-1');
  fixture.componentRef.setInput('viewer', viewer);
  fixture.componentRef.setInput('vitals', pensantusVitals({ characterId: 'char-1' }));
  fixture.detectChanges();
  return { fixture, client, el: fixture.nativeElement as HTMLElement };
}

async function settle(fixture: ReturnType<typeof setup>['fixture']) {
  await fixture.whenStable();
  await Promise.resolve();
  fixture.detectChanges();
}

describe('FavorPanel', () => {
  it('reads to the owner what it is, from whom and how to use it, with no button to pass it (the pass is not built)', async () => {
    const { fixture, el } = setup('owner', ['char-1']);
    await settle(fixture);
    expect(flat(el.querySelector('h2'))).toBe('starInspiração');
    expect(flat(el.querySelector('.favor__text'))).toBe(
      'Do mestre. Vantagem num d20 seu: marque “Usar a Inspiração” antes de rolar.',
    );
    expect(el.querySelector('button')).toBeNull();
  });

  it("says the title alone to the master: the sentence is the player's", async () => {
    const { fixture, el } = setup('master', ['char-1']);
    await settle(fixture);
    expect(el.querySelector('h2')).not.toBeNull();
    expect(el.querySelector('.favor__text')).toBeNull();
  });

  it('draws nothing when the character has none, whoever the read says has it', async () => {
    const { fixture, el } = setup('owner', ['char-2']);
    await settle(fixture);
    expect(el.querySelector('section')).toBeNull();
  });

  it('draws nothing when the read fails', async () => {
    const { fixture, client, el } = setup('owner', ['char-1']);
    await settle(fixture);
    client.holders = () => {
      throw new Error('down');
    };
    fixture.componentRef.setInput(
      'vitals',
      pensantusVitals({ characterId: 'char-1', revision: 5 }),
    );
    fixture.detectChanges();
    await settle(fixture);
    expect(el.querySelector('section')).toBeNull();
  });

  it('reads again when the live numbers change: a grant tells nothing else', async () => {
    const { fixture, client, el } = setup('owner', []);
    await settle(fixture);
    expect(el.querySelector('section')).toBeNull();
    const reads = client.reads.length;
    client.holders = [{ characterId: 'char-1', name: 'Pensantus' }];
    fixture.componentRef.setInput(
      'vitals',
      pensantusVitals({ characterId: 'char-1', revision: 2 }),
    );
    fixture.detectChanges();
    await settle(fixture);
    expect(client.reads.length).toBeGreaterThan(reads);
    expect(el.querySelector('section')).not.toBeNull();
  });
});

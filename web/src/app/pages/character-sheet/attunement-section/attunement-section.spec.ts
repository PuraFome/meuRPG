import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { describe, expect, it, vi } from 'vitest';

import { type AttunementEntryVm, AttunementClient } from '../../../core/characters/attunement';

import { AttunementSection } from './attunement-section';

const entry = (o: Partial<AttunementEntryVm> & { id: string }): AttunementEntryVm => ({
  key: `item:${o.id}`,
  namePt: o.id,
  kind: 'magic',
  attuned: false,
  asks: true,
  restrictionPt: '',
  restriction: 'none',
  sameItemAttuned: false,
  ...o,
});

function setup(
  set: AttunementClient['set'],
  entries: AttunementEntryVm[],
  viewer: 'owner' | 'master' = 'owner',
) {
  TestBed.configureTestingModule({ providers: [{ provide: AttunementClient, useValue: { set } }] });
  const fixture = TestBed.createComponent(AttunementSection);
  fixture.componentRef.setInput('attunement', {
    entries,
    attuned: entries.filter((e) => e.attuned).length,
  });
  fixture.componentRef.setInput('viewer', viewer);
  fixture.componentRef.setInput('characterName', 'Tor');
  fixture.componentRef.setInput('windowOpen', true);
  fixture.componentRef.setInput('campaignId', 'c1');
  fixture.componentRef.setInput('characterId', 'p1');
  fixture.componentRef.setInput('revision', 7);
  fixture.detectChanges();
  return fixture;
}
const text = (f: ReturnType<typeof setup>): string =>
  (f.nativeElement as HTMLElement).textContent ?? '';
const button = (f: ReturnType<typeof setup>, id: string): HTMLButtonElement =>
  (f.nativeElement as HTMLElement).querySelector(`[data-entry-id="${id}"]`)!;

describe('AttunementSection', () => {
  it('asks the server with the entry id and the held revision, then says it and tells the page', async () => {
    const set = vi.fn().mockResolvedValue(undefined);
    const f = setup(set, [entry({ id: 'm1', namePt: 'Manto élfico' })]);
    const changed = vi.fn();
    f.componentInstance.changed.subscribe(changed);
    button(f, 'm1').click();
    await f.whenStable();
    f.detectChanges();
    expect(set).toHaveBeenCalledWith('c1', 'p1', 'm1', true, 7);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(text(f)).toContain('Manto élfico sintonizado. Sintonizados: 1 de 3.');
  });

  it('does nothing for a button the section knows will be refused, and says why under the row', () => {
    const set = vi.fn();
    const f = setup(set, [
      entry({
        id: 'm2',
        namePt: 'Pérola do poder',
        restrictionPt: 'Só para um conjurador',
        restriction: 'notMet',
      }),
    ]);
    expect(button(f, 'm2').getAttribute('aria-disabled')).toBe('true');
    button(f, 'm2').click();
    expect(set).not.toHaveBeenCalled();
    expect(text(f)).toContain('Só para um conjurador: você não cumpre.');
  });

  it('shows a refusal as an alert in the app words and keeps the row', async () => {
    const set = vi.fn().mockRejectedValue(new ConnectError('stale', Code.Aborted));
    const f = setup(set, [entry({ id: 'm1', namePt: 'Manto élfico' })]);
    button(f, 'm1').click();
    await f.whenStable();
    f.detectChanges();
    const alert = (f.nativeElement as HTMLElement).querySelector('[role="alert"]');
    expect(alert?.textContent).toContain('Não foi possível sintonizar Manto élfico.');
    expect(button(f, 'm1')).toBeTruthy();
  });

  it('puts the master in the dialog title with the name', () => {
    const f = setup(vi.fn(), [], 'master');
    expect(text(f)).toContain('Sintonização · Tor');
  });
});

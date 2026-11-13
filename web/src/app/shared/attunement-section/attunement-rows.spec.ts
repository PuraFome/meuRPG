import { describe, expect, it } from 'vitest';

import type { AttunementEntryVm, AttunementVm } from '../../core/characters/attunement';

import { sectionOf } from './attunement-rows';

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
const vm = (entries: AttunementEntryVm[]): AttunementVm => ({
  entries,
  attuned: entries.filter((e) => e.asks && e.attuned).length,
});
const owner = { viewer: 'owner', name: 'Tor', open: true, dead: false } as const;
const master = { viewer: 'master', name: 'Tor', open: false, dead: false } as const;

describe('sectionOf', () => {
  it('says what a restriction the character does not meet reads like, to the owner and to the master', () => {
    const a = vm([
      entry({ id: 'pearl', restrictionPt: 'Só para um conjurador', restriction: 'notMet' }),
    ]);
    const o = sectionOf(a, owner).asking[0];
    expect(o).toMatchObject({
      button: 'dashed',
      reason: 'Só para um conjurador: você não cumpre.',
    });
    const m = sectionOf(a, master).asking[0];
    expect(m).toMatchObject({
      button: 'on',
      reason:
        'Só para um conjurador: Tor não cumpre. Como mestre, você pode sintonizar mesmo assim.',
    });
  });

  it('leaves an alignment the sheet does not say to the master, and a circumstance as plain text', () => {
    const a = vm([
      entry({
        id: 'talisman',
        restrictionPt: 'Só para criatura de alinhamento bom',
        restriction: 'unknown',
      }),
      entry({ id: 'stars', restrictionPt: 'Só ao ar livre, à noite', restriction: 'met' }),
    ]);
    const [t, s] = sectionOf(a, owner).asking;
    expect(t).toMatchObject({ button: 'dashed' });
    expect(t.reason).toContain('Só o mestre pode sintonizar.');
    expect(s).toMatchObject({
      button: 'on',
      reason: 'Só ao ar livre, à noite.',
      reasonIcon: 'dark_mode',
    });
    expect(sectionOf(a, master).asking[0].reason).toContain('Você decide.');
  });

  it('at three of three the owner may only end, and the limit line differs for the master', () => {
    const a = vm([
      entry({ id: 'a', attuned: true }),
      entry({ id: 'b', attuned: true }),
      entry({ id: 'c', attuned: true }),
      entry({ id: 'cloak' }),
    ]);
    const o = sectionOf(a, owner);
    expect(o.asking[0].button).toBe('dashed');
    expect(o.attuned.every((r) => r.button === 'on')).toBe(true);
    expect(o.limitLine).toBe(
      'Limite de 3 itens. Encerre uma agora; sintonizar outro fica para o próximo descanso.',
    );
    expect(o.lead).toBe('Neste descanso você pode encerrar um item.');
    expect(sectionOf(a, master).limitLine).toBe(
      'Limite de 3 itens. Encerre um antes de sintonizar outro.',
    );
  });

  it('gives the owner no button without a window, and a dead character none at all but the master ends', () => {
    const a = vm([entry({ id: 'cloak' }), entry({ id: 'ring', attuned: true })]);
    const closed = sectionOf(a, { ...owner, open: false });
    expect([...closed.asking, ...closed.attuned].every((r) => r.button === 'none')).toBe(true);
    expect(closed.lead).toContain('foi usada');
    const dead = sectionOf(a, { ...owner, dead: true });
    expect(dead.lead).toBe('Morto: os itens e as sintonizações ficam como estavam.');
    const masterDead = sectionOf(a, { ...master, dead: true });
    expect(masterDead.asking).toEqual([]);
    expect(masterDead.attuned.map((r) => [r.entry.id, r.button])).toEqual([['ring', 'on']]);
  });

  it('marks the second copy of an item and counts nothing for an item that asks for nothing', () => {
    const a = vm([
      entry({ id: 'ring2', sameItemAttuned: true }),
      entry({ id: 'bag', asks: false }),
    ]);
    const s = sectionOf(a, owner);
    expect(s.asking).toHaveLength(1);
    expect(s.asking[0]).toMatchObject({
      button: 'dashed',
      reason: 'Já sintonizado a outro igual.',
    });
    expect(s.count).toBe('Sintonizados: 0 de 3');
    expect(sectionOf(vm([]), owner).empty).toBe('Nada nesta ficha pede sintonização.');
  });
});

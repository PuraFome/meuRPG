import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  DisabledReasonCode,
  SpellOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { ActionGroups } from './action-groups';

/** A spell option as `GetTurnOptions` sends it (name, circle, economy, and why it is off). */
function spell(key: string, namePt: string, level: number, over: { enabled?: boolean; reason?: DisabledReasonCode; economy?: ActionEconomy } = {}) {
  const enabled = over.enabled ?? true;
  return create(SpellOptionSchema, {
    spell: { key, name: namePt, namePt, level, schoolNamePt: 'Encantamento' },
    economy: over.economy ?? ActionEconomy.ACTION,
    enabled,
    reason: enabled ? undefined : { code: over.reason ?? DisabledReasonCode.NO_SLOT, minLevel: level },
  });
}

describe('ActionGroups: the spells (E8-02)', () => {
  function setup(spells: ReturnType<typeof spell>[], slots = { usage: [] as { level: number; total: number; used: number }[], pact: null }) {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, { spells }));
    fixture.componentRef.setInput('own', create(CombatantSchema, { movementLeftFt: 25, speedFt: 25 }));
    fixture.componentRef.setInput('slots', slots);
    const described: { key: string; name: string }[] = [];
    const cast: string[] = [];
    fixture.componentInstance.describe.subscribe((d) => described.push(d));
    fixture.componentInstance.cast.subscribe((k) => cast.push(k));
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { el, described, cast };
  }

  // The order the server sends: what can be cast now first, then by circle and name.
  const SERVER_ORDER = [
    spell('spell:minor-illusion', 'Ilusão Menor', 0),
    spell('spell:mage-armor', 'Armadura Arcana', 1),
    spell('spell:sleep', 'Sono', 1),
    spell('spell:shield', 'Escudo Arcano', 1, { enabled: false, reason: DisabledReasonCode.REACTION_ONLY_WHEN_HIT, economy: ActionEconomy.REACTION }),
    spell('spell:scorching-ray', 'Raio Ardente', 2, { enabled: false }),
    spell('spell:web', 'Teia', 2, { enabled: false }),
  ];

  // The spell rows: the Movimento row has no name.
  const rows = (el: HTMLElement) => ([...el.querySelectorAll('app-action-row')] as HTMLElement[]).filter((r) => r.querySelector('.row__name'));
  const name = (row: HTMLElement) => row.querySelector('.row__name')!.textContent!.trim();

  it('lists the spells in the order the server sent, every economy in one list', () => {
    const { el } = setup(SERVER_ORDER);
    expect(rows(el).map(name)).toEqual(['Ilusão Menor', 'Armadura Arcana', 'Sono', 'Escudo Arcano', 'Raio Ardente', 'Teia']);
  });

  it('gives each spell the "?" with its name, 44 px, whether or not it can be cast', () => {
    const { el } = setup(SERVER_ORDER);
    for (const row of rows(el)) {
      const help = row.querySelector<HTMLButtonElement>('app-spell-help button')!;
      expect(help.getAttribute('aria-label')).toBe(`Detalhes de ${name(row)}`);
      expect(help.disabled).toBe(false);
    }
  });

  it('opens the details with the spell key and name', () => {
    const { el, described } = setup(SERVER_ORDER);
    rows(el)[2].querySelector<HTMLButtonElement>('app-spell-help button')!.click();
    expect(described).toEqual([{ key: 'spell:sleep', name: 'Sono' }]);
    // A spell that is off has its "?" too.
    rows(el)[5].querySelector<HTMLButtonElement>('app-spell-help button')!.click();
    expect(described[1]).toEqual({ key: 'spell:web', name: 'Teia' });
  });

  it('puts the circle in a tag on its own line, and "Reação" next to it for Escudo', () => {
    const { el } = setup(SERVER_ORDER);
    const tags = (row: HTMLElement) => [...row.querySelectorAll('.row__tags .row__pill')].map((t) => t.textContent!.trim());
    expect(tags(rows(el)[0])).toEqual(['Truque']);
    expect(tags(rows(el)[2])).toEqual(['1º círculo']);
    expect(tags(rows(el)[3])).toEqual(['1º círculo', 'Reação']);
    expect(el.querySelector('.row__name .row__pill')).toBeNull();
  });

  it('Escudo Arcano on the own turn keeps a dashed, disabled "Conjurar" and says "Só fora da sua vez"', () => {
    const { el, cast } = setup(SERVER_ORDER);
    const shield = rows(el)[3];
    const button = shield.querySelector<HTMLButtonElement>('button.row__btn')!;
    expect(button.getAttribute('aria-disabled')).toBe('true');
    expect(button.classList).toContain('row__btn--off');
    button.click();
    expect(cast).toEqual([]);
    expect(shield.querySelector('.row__why')!.textContent).toContain('Só fora da sua vez');
  });

  it('repeats only "Sem espaço" on a spell without a slot, and the slot rows above are the explanation', () => {
    const { el } = setup(SERVER_ORDER, {
      usage: [
        { level: 1, total: 4, used: 4 },
        { level: 2, total: 2, used: 2 },
      ],
      pact: null,
    });
    const web = rows(el)[5];
    expect(web.querySelector('.row__why')!.textContent!.trim()).toContain('Sem espaço');
    expect(web.querySelector('.row__why')!.textContent).not.toContain('círculo');
    const slotRows = [...el.querySelectorAll('.slots__row')].map((r) => [
      r.querySelector('.slots__title')!.textContent!.trim(),
      r.querySelector('.slots__text')!.textContent!.trim(),
    ]);
    expect(slotRows).toEqual([
      ['1º\u00a0círculo', '0 livres de 4'],
      ['2º\u00a0círculo', '0 livres de 2'],
    ]);
  });

  it('casts a spell that can be cast, and not one that cannot', () => {
    const { el, cast } = setup(SERVER_ORDER);
    rows(el)[2].querySelector<HTMLButtonElement>('button.row__btn')!.click();
    rows(el)[5].querySelector<HTMLButtonElement>('button.row__btn')!.click();
    expect(cast).toEqual(['spell:sleep']);
  });
});

import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  AttackKind,
  AttackOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { GearVm } from '../../../../core/characters/gear';
import { GearFlow } from '../../../../core/combat/gear-flow';
import { ActionGroups } from './action-groups';

const sword: GearVm = {
  key: 'equipment:longsword',
  namePt: 'Espada longa',
  kind: 'weapon',
  equipped: true,
  twoHanded: false,
  armorClassIfToggled: 0,
  count: 1,
};

describe('ActionGroups: the magic bonus chip on an attack row', () => {
  const attack = (
    over: { silvered?: boolean; magical?: boolean; kind?: AttackKind; key?: string } = {},
  ) =>
    create(AttackOptionSchema, {
      attack: {
        key: over.key ?? sword.key,
        name: 'Longsword',
        namePt: 'Espada longa',
        kind: over.kind ?? AttackKind.WEAPON,
        silvered: over.silvered ?? false,
        magical: over.magical ?? false,
        attackBonus: 7,
        damage: '1d8 + 4',
      },
      enabled: true,
    });

  async function row(gear: GearVm[], opt = attack()): Promise<HTMLElement> {
    const fixture = TestBed.createComponent(ActionGroups);
    const flow = TestBed.runInInjectionContext(
      () =>
        new GearFlow({
          client: {
            read: () => Promise.resolve({ gear, armorClass: 16, shieldIdle: false, revision: 1 }),
            set: () => Promise.resolve(),
          },
          campaignId: () => 'c1',
          characterId: () => 'ch1',
          signature: () => '',
        }),
    );
    await flow.load();
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, { attacks: [opt] }));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 25, speedFt: 25 }),
    );
    fixture.componentRef.setInput('gearFlow', flow);
    fixture.detectChanges();
    return (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>('app-action-row')!;
  }
  const chip = (r: HTMLElement) => r.querySelector('.row__head app-bonus-chip .bonus-chip');

  it('names a counting +N beside the attack, with no "Mágica" mark for the weapon', async () => {
    const r = await row(
      [{ ...sword, bonus: 1, bonusCounts: true }],
      attack({ magical: true, silvered: true }),
    );
    expect(chip(r)!.getAttribute('aria-label')).toBe('Bônus mágico +1, já somado');
    const marks = Array.from(r.querySelectorAll('.weapon-tags li')).map((li) => li.textContent);
    expect(marks).toEqual([expect.stringContaining('Prateada')]);
  });

  it('keeps "Mágica" and shows no chip without a counting bonus', async () => {
    for (const piece of [sword, { ...sword, bonus: 1, bonusCounts: false }]) {
      const r = await row([piece], attack({ magical: true }));
      expect(chip(r)).toBeNull();
      expect(r.querySelector('.weapon-tags')!.textContent).toContain('Mágica');
    }
  });

  it('shows no chip on a spell', async () => {
    const r = await row(
      [{ ...sword, bonus: 1, bonusCounts: true }],
      attack({ kind: AttackKind.SPELL, key: 'spell:fire-bolt' }),
    );
    expect(chip(r)).toBeNull();
  });

  it('marks the name line as one line, with the name as the chip reference and the chips as chips', async () => {
    const r = await row([{ ...sword, bonus: 1, bonusCounts: true }], attack({ silvered: true }));
    const head = r.querySelector('.row__head')!;
    expect(head.hasAttribute('data-chip-row')).toBe(true);
    expect(head.hasAttribute('data-one-line')).toBe(true);
    expect(head.querySelector('.row__name')!.hasAttribute('data-chip-ref')).toBe(true);
    expect(head.querySelector('app-bonus-chip')!.hasAttribute('data-chip')).toBe(true);
    expect(head.querySelectorAll('.weapon-tags__chip[data-chip]')).toHaveLength(1);
  });
});

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
    over: {
      silvered?: boolean;
      magical?: boolean;
      kind?: AttackKind;
      key?: string;
      magicBonus?: number;
    } = {},
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
      magicBonus: over.magicBonus ?? 0,
      enabled: true,
    });

  async function rows(opts = [attack()], gear: GearVm[] | null = null): Promise<HTMLElement[]> {
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, { attacks: opts }));
    fixture.componentRef.setInput(
      'own',
      create(CombatantSchema, { movementLeftFt: 25, speedFt: 25 }),
    );
    if (gear) {
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
      fixture.componentRef.setInput('gearFlow', flow);
    }
    fixture.detectChanges();
    return Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll<HTMLElement>('app-action-row'),
    );
  }
  const chip = (r: HTMLElement) => r.querySelector('.row__head app-bonus-chip .bonus-chip');

  it('names the +N of the option beside the attack, with no "Mágica" mark for the weapon', async () => {
    const [r] = await rows([attack({ magical: true, silvered: true, magicBonus: 1 })]);
    expect(chip(r)!.getAttribute('aria-label')).toBe('Bônus mágico +1, já somado');
    const marks = Array.from(r.querySelectorAll('.weapon-tags li')).map((li) => li.textContent);
    expect(marks).toEqual([expect.stringContaining('Prateada')]);
  });

  it('reads the field and nothing else: a gear bonus with a 0 field gives no chip', async () => {
    const [r] = await rows(
      [attack({ magical: true })],
      [{ ...sword, bonus: 2, bonusCounts: true }],
    );
    expect(chip(r)).toBeNull();
    expect(r.querySelector('.weapon-tags')!.textContent).toContain('Mágica');
  });

  it('shows the +2 of the field with no gear read', async () => {
    const [r] = await rows([attack({ magicBonus: 2 })]);
    expect(chip(r)!.textContent).toContain('+2');
  });

  it('gives a second weapon without a bonus no chip', async () => {
    const [a, b] = await rows([
      attack({ magicBonus: 1 }),
      attack({ key: 'equipment:dagger', magical: true }),
    ]);
    expect(chip(a)).not.toBeNull();
    expect(chip(b)).toBeNull();
    expect(b.querySelector('.weapon-tags')!.textContent).toContain('Mágica');
  });

  it('shows no chip on a spell', async () => {
    const [r] = await rows([attack({ kind: AttackKind.SPELL, key: 'spell:fire-bolt' })]);
    expect(chip(r)).toBeNull();
  });

  it('puts the marks before the chip in the row', async () => {
    const [r] = await rows([attack({ silvered: true, magicBonus: 1 })]);
    const head = r.querySelector('.row__head')!;
    const tags = head.querySelector('app-weapon-tags')!;
    const bonus = head.querySelector('app-bonus-chip')!;
    expect(tags.compareDocumentPosition(bonus) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('marks the name line as one line, with the name as the chip reference and the chips as chips', async () => {
    const [r] = await rows([attack({ silvered: true, magicBonus: 1 })]);
    const head = r.querySelector('.row__head')!;
    expect(head.hasAttribute('data-chip-row')).toBe(true);
    expect(head.hasAttribute('data-one-line')).toBe(true);
    expect(head.querySelector('.row__name')!.hasAttribute('data-chip-ref')).toBe(true);
    expect(head.querySelector('app-bonus-chip')!.hasAttribute('data-chip')).toBe(true);
    expect(head.querySelectorAll('.weapon-tags__chip[data-chip]')).toHaveLength(1);
  });

  describe('the legend under the tab', () => {
    let host: HTMLElement;
    const legendOf = () => host.querySelector('app-chip-legend ul');
    const render = async (opts: ReturnType<typeof attack>[], gear: GearVm[] | null = null) => {
      const els = await rows(opts, gear);
      host = els[0]?.closest('app-action-groups') ?? document.body;
      return els;
    };
    const items = () =>
      Array.from(legendOf()?.querySelectorAll('li') ?? []).map((li) =>
        (li.textContent ?? '')
          .replace(/auto_awesome|dark_mode|diamond|\+\d/g, '')
          .replace(/\s+/g, ' ')
          .trim(),
      );

    it('is not drawn when no row carries a mark or a +N', async () => {
      await render([attack()]);
      expect(legendOf()).toBeNull();
    });

    it('lists what the rows show, in the sheet legend order: the mark, then the bonus words', async () => {
      await render([attack({ silvered: true, magical: true, magicBonus: 1 })]);
      expect(items()).toEqual(['Prateada', 'Bônus mágico, já somado']);
    });

    it('lists "Mágica" only for a weapon whose +N does not take its place', async () => {
      await render([attack({ magical: true })]);
      expect(items()).toEqual(['Mágica']);
    });

    it('adds what the fold draws: a carried weapon with its bonus dashed', async () => {
      await render(
        [attack()],
        [
          {
            ...sword,
            equipped: false,
            bonus: 1,
            bonusCounts: false,
            marks: { silvered: false, adamantine: true, magical: false },
          },
        ],
      );
      expect(items()).toEqual(['Adamantina', 'Bônus mágico, não conta agora']);
    });

    it('is hidden from a screen reader, which already hears each chip', async () => {
      await render([attack({ magicBonus: 1 })]);
      expect(legendOf()!.getAttribute('aria-hidden')).toBe('true');
      expect(legendOf()!.hasAttribute('aria-label')).toBe(false);
    });

    it('is the last thing in the "Ataques" panel, after the fold', async () => {
      await render([attack({ magicBonus: 1 })], [sword]);
      const panel = host.querySelector('[role="tabpanel"]')!;
      expect(panel.lastElementChild!.tagName.toLowerCase()).toBe('app-attack-legend');
    });
  });
});

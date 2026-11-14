import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { CombatantSchema } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  EffectLabelSchema,
  EffectModifierKind,
  EffectModifierSchema,
  WeaponHitDamageSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import {
  AttackKind,
  AttackOptionSchema,
  TurnOptionsSchema,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { lastingEffect } from '../../../../core/effects/effects-testing';
import { ActionGroups } from './action-groups';

const plain = (t: string | null | undefined) => (t ?? '').replace(/\s+/g, ' ').trim();

const sword = create(AttackOptionSchema, {
  enabled: true,
  attack: {
    key: 'attack:shortsword',
    name: 'Shortsword',
    namePt: 'Espada curta',
    kind: AttackKind.WEAPON,
    attackBonus: 5,
    rangeFt: 5,
    damage: '1d6 + 3',
    damageTypePt: 'perfurante',
  },
});

function mark(id: string, over: object = {}, hit: object = {}) {
  return lastingEffect({
    id,
    sourceKey: 'spell:hunters-sign',
    sourceNamePt: 'Sinal da Presa',
    casterId: 'me',
    targetIds: ['me'],
    concentration: true,
    roundsLeft: 580,
    modifiers: [
      create(EffectModifierSchema, {
        kind: EffectModifierKind.WEAPON_HIT_DAMAGE,
        hit: create(WeaponHitDamageSchema, { trigger: 'marked_target', die: 6, dice: 1, ...hit }),
      }),
    ],
    ...over,
  });
}

describe('ActionGroups: the card of the effect waiting for the next hit (#577)', () => {
  function setup(effects: ReturnType<typeof mark>[]) {
    const own = create(CombatantSchema, { id: 'me', label: 'Jabuti', mine: true, effects });
    const wight = create(CombatantSchema, {
      id: 'w',
      label: 'Wight',
      effectLabels: [create(EffectLabelSchema, { effectId: 'm', textPt: 'Sinal da Presa' })],
    });
    const fixture = TestBed.createComponent(ActionGroups);
    fixture.componentRef.setInput('options', create(TurnOptionsSchema, { attacks: [sword] }));
    fixture.componentRef.setInput('own', own);
    fixture.componentRef.setInput('combatants', [own, wight]);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const cards = (el: HTMLElement) =>
    Array.from(el.querySelectorAll<HTMLElement>('[data-testid="hit-effect-card"]'));

  it('puts the card at the top of Ataques, under the economy row, with the marked creature named', () => {
    const el = setup([mark('a')]);
    const [card] = cards(el);
    const panel = el.querySelector<HTMLElement>('[role="tabpanel"]');
    expect(card.getAttribute('role')).toBe('group');
    expect(panel?.firstElementChild).toBe(card.parentElement);
    expect(plain(card.querySelector('.hit__title')?.textContent)).toBe('Sinal da Presa no Wight');
    expect(plain(card.querySelector('.hit__line')?.textContent)).toBe(
      'Cada golpe com arma no Wight: +1d6 do tipo da arma. 58 min restantes.',
    );
    expect(plain(card.querySelector('.hit__chip')?.textContent)).toContain('Concentração');
    const economy = el.querySelector('[data-testid="turn-economy"]');
    expect(economy?.compareDocumentPosition(card)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('has no concentration chip for an effect that does not need it', () => {
    const el = setup([mark('a', { concentration: false })]);
    expect(cards(el)[0].querySelector('.hit__chip')).toBeNull();
  });

  it('draws nothing for a spent effect, another caster, or an effect with no hit', () => {
    const el = setup([
      mark('spent', { spent: true }),
      mark('other', { casterId: 'x' }),
      lastingEffect({ id: 'bless', casterId: 'me' }),
    ]);
    expect(cards(el)).toHaveLength(0);
  });
});

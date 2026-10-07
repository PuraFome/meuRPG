import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { SpellCatalog } from '../../../../core/combat/spell-catalog';
import { CastSheet, type CastSheetData } from './cast-sheet';

/** A concentration spell cast over another one says what it ends, and what goes with it (the combat's own data). */
describe('CastSheet: ending another concentration', () => {
  const salvia = (over: Record<string, unknown> = {}) =>
    combatant({ id: 's', label: 'Sálvia', kind: CombatantKind.PLAYER, characterId: 'sc', mine: true, ...over });
  const wolf = (n: number) =>
    combatant({ id: `w${n}`, label: `Lobo atroz ${n}`, kind: CombatantKind.CREATURE, characterId: '', ownerCharacterId: 'sc', summonGroupId: 'cast', monsterKey: 'monster:dire-wolf', monsterNamePt: 'Lobo atroz' });

  function open(combatants: ReturnType<typeof combatant>[], concentration = true) {
    TestBed.resetTestingModule();
    const data = {
      campaignId: 'c',
      encounterId: 'enc',
      casterId: 's',
      round: 2,
      spellKey: 'spell:entangle',
      name: 'Constrição',
      level: 1,
      concentration,
      economy: ActionEconomy.ACTION,
      slots: [],
      usage: [],
      pact: null,
      targets: undefined,
      shieldFree: null,
      shieldName: '',
      attackBonus: 0,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      state: { encounter: signal(encounter({ combatants })) },
    } as unknown as CastSheetData;
    TestBed.configureTestingModule({
      providers: [
        { provide: CombatClient, useValue: {} },
        { provide: SpellCatalog, useValue: { details: () => Promise.resolve(null) } },
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close: vi.fn() } },
      ],
    });
    const fixture = TestBed.createComponent(CastSheet);
    fixture.detectChanges();
    return (fixture.nativeElement as HTMLElement).querySelector('.ends')?.textContent?.replace(/\s+/g, ' ').trim() ?? null;
  }

  it('says nothing when the caster is not concentrating', () => {
    expect(open([salvia()])).toBeNull();
  });

  it('says nothing when the spell needs no concentration, or is the one already held', () => {
    expect(open([salvia({ concentrationSpell: 'spell:web', concentrationSpellNamePt: 'Teia' })], false)).toBeNull();
    expect(open([salvia({ concentrationSpell: 'spell:entangle', concentrationSpellNamePt: 'Constrição' })])).toBeNull();
  });

  it('says what the other concentration is when the caster holds one that keeps no creatures', () => {
    expect(open([salvia({ concentrationSpell: 'spell:web', concentrationSpellNamePt: 'Teia' })])).toContain('Constrição encerra a concentração em Teia.');
    expect(open([salvia({ concentrationSpell: 'spell:web', concentrationSpellNamePt: 'Teia' })])).not.toContain('somem');
  });

  it('adds the creatures that go with it, from the combatants with this owner and a summon group', () => {
    const text = open([salvia({ concentrationSpell: 'spell:conjure-animals', concentrationSpellNamePt: 'Conjurar Animais' }), wolf(1), wolf(2)]);
    expect(text).toContain('Constrição encerra a concentração em Conjurar Animais.');
    expect(text).toContain('Os 2 Lobos atrozes somem.');
  });
});

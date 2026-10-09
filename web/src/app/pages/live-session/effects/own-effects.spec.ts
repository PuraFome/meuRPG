import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';

import { type Encounter, EncounterStatus } from '../../../../gen/meurpg/play/v1/combat_pb';
import {
  CharacterEffectSchema,
  LastingEffectSchema,
} from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { EffectsClient } from '../../../core/effects/effects-client';
import { OwnEffects } from './own-effects';

describe('OwnEffects', () => {
  const api = { listCharacterEffects: vi.fn() };
  const outside = create(CharacterEffectSchema, {
    id: 'c1',
    sourceNamePt: 'Bênção',
    durationTextPt: 'dura 1 minuto',
    tagsPt: ['+1d4'],
    conditionNamesPt: [],
  });

  function setup(over: { master?: boolean; enc?: Encounter | null } = {}) {
    TestBed.configureTestingModule({ providers: [{ provide: EffectsClient, useValue: api }] });
    const isMaster = signal(over.master ?? false);
    const enc = signal<Encounter | null>(over.enc ?? null);
    const tick = signal(0);
    const own = TestBed.runInInjectionContext(
      () =>
        new OwnEffects({
          campaignId: signal('camp'),
          isMaster,
          encounter: enc,
          tick,
        }),
    );
    return { own, enc, tick, isMaster };
  }
  const settle = async () => {
    TestBed.tick();
    await Promise.resolve();
    await Promise.resolve();
    TestBed.tick();
  };

  beforeEach(() => {
    api.listCharacterEffects.mockReset();
    api.listCharacterEffects.mockResolvedValue([outside]);
  });

  it('reads the effects outside a combat, and again when something moves', async () => {
    const { own, tick } = setup();
    await settle();
    expect(api.listCharacterEffects).toHaveBeenCalledWith('camp');
    expect(own.cards().map((c) => [c.name, c.clock])).toEqual([['Bênção', 'Dura 1 minuto']]);
    tick.update((n) => n + 1);
    await settle();
    expect(api.listCharacterEffects).toHaveBeenCalledTimes(2);
  });

  it('takes the cards and the conditions of the own combatant while a combat runs, and reads nothing', async () => {
    const mine = combatant({
      id: 'me',
      label: 'Brisa',
      mine: true,
      conditionNamesPt: ['Paralisado'],
      effects: [
        create(LastingEffectSchema, {
          id: 'e1',
          sourceNamePt: 'Imobilizar Pessoa',
          originPt: 'De alguém que você não vê',
        }),
      ],
    } as never);
    const { own } = setup({
      enc: encounter({ status: EncounterStatus.ACTIVE, combatants: [mine] }),
    });
    await settle();
    expect(api.listCharacterEffects).not.toHaveBeenCalled();
    expect(own.cards().map((c) => c.name)).toEqual(['Imobilizar Pessoa']);
    expect(own.conditions()).toEqual(['Paralisado']);
  });

  it('reads nothing for the master, who has the panel of the combat', async () => {
    const { own } = setup({ master: true });
    await settle();
    expect(api.listCharacterEffects).not.toHaveBeenCalled();
    expect(own.cards()).toEqual([]);
  });

  it('keeps what it had when a read fails', async () => {
    const { own, tick } = setup();
    await settle();
    api.listCharacterEffects.mockRejectedValue(new Error('offline'));
    tick.update((n) => n + 1);
    await settle();
    expect(own.cards()).toHaveLength(1);
  });
});

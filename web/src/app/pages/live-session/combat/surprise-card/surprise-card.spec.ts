import { TestBed } from '@angular/core/testing';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  HiderTotalSchema,
  SurpriseSuggestionReason,
  SurpriseSuggestionSchema,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import { FakeContestClient, textOf } from '../../../../core/combat/contest-testing';
import { SurpriseCard } from './surprise-card';

const suggestion = (over: object) =>
  create(SurpriseSuggestionSchema, { passivePerception: 9, ...over });

const beats = suggestion({
  combatantId: 'g1',
  suggested: true,
  reason: SurpriseSuggestionReason.HIDERS_BEAT,
  hiders: [create(HiderTotalSchema, { combatantId: 'b', stealthTotal: 19, beats: true })],
});

function setup(suggestions = [beats]) {
  const api = new FakeContestClient();
  api.suggestions = suggestions;
  const e = encounter({
    status: 1,
    combatants: [
      combatant({ id: 'b', label: 'Brisa', kind: CombatantKind.PLAYER }),
      combatant({ id: 'nael', label: 'Nael', kind: CombatantKind.PLAYER }),
      combatant({ id: 'g1', label: 'Goblin 1' }),
    ],
  });
  api.encounterAnswer = encounter({ ...e, revision: 9 } as never);
  const state = new CombatState();
  state.encounter.set(e);
  TestBed.configureTestingModule({ providers: [{ provide: ContestClient, useValue: api.as() }] });
  const fixture = TestBed.createComponent(SurpriseCard);
  const ref = fixture.componentRef;
  ref.setInput('encounter', e);
  ref.setInput('campaignId', 'camp');
  ref.setInput('state', state);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const settle = async () => {
    for (let i = 0; i < 4; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  const button = (name: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      textOf(b).includes(name),
    );
  return { api, fixture, el, settle, button, state };
}

describe('SurpriseCard, "Quem está surpreso?"', () => {
  it('reads the suggestion of the app and leaves the decision to the master', async () => {
    const { el, settle } = setup([
      beats,
      suggestion({
        combatantId: 'nael',
        passivePerception: 12,
        reason: SurpriseSuggestionReason.NO_HIDERS,
      }),
    ]);
    await settle();
    expect(textOf(el.querySelector('h2')!)).toBe('Quem está surpreso?');
    expect(textOf(el)).toContain(
      'Sugestão: a Furtividade de cada um que se esconde contra a Percepção passiva de cada criatura. Você decide.',
    );
    expect(Array.from(el.querySelectorAll('.row')).map((r) => textOf(r))).toEqual([
      'Goblin 1 Percepção passiva 9 · Furtividade de Brisa 19 vence Sugerido Surpreso',
      'Nael Percepção passiva 12 · Não está escondido: nota Surpreso',
    ]);
    const boxes = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
    expect(boxes.map((b) => b.checked)).toEqual([false, false]);
  });

  it('marks a creature, player characters included, with its own key and reads the suggestion again', async () => {
    const { api, el, settle, state } = setup([
      suggestion({ combatantId: 'nael', passivePerception: 12 }),
    ]);
    await settle();
    api.suggestions = [suggestion({ combatantId: 'nael', passivePerception: 12, surprised: true })];
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    expect(api.surprised).toEqual([
      { combatantId: 'nael', surprised: true, key: expect.any(String) },
    ]);
    expect(state.encounter()?.revision).toBe(9);
    expect(el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.checked).toBe(true);
    expect(textOf(el)).toContain('Nael: surpreso.');
  });

  it('unmarks the one that was marked', async () => {
    const { api, el, settle } = setup([
      suggestion({ combatantId: 'nael', passivePerception: 12, surprised: true }),
    ]);
    await settle();
    api.suggestions = [suggestion({ combatantId: 'nael', passivePerception: 12 })];
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    expect(api.surprised[0].surprised).toBe(false);
  });

  it('"Marcar os sugeridos" marks only the suggested ones that are not marked yet', async () => {
    const { api, settle, button } = setup([
      beats,
      suggestion({ combatantId: 'nael', passivePerception: 12 }),
    ]);
    await settle();
    button('Marcar os sugeridos')!.click();
    await settle();
    expect(api.surprised.map((s) => s.combatantId)).toEqual(['g1']);
  });

  it('says the refusal of the server (the combat already began)', async () => {
    const { api, el, settle } = setup();
    await settle();
    api.error = new ConnectError('begun', Code.FailedPrecondition);
    el.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
    await settle();
    expect(el.querySelector('[role="alert"]')).toBeTruthy();
  });
});

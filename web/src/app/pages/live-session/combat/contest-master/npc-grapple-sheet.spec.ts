import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatantKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  CheckOptionSchema,
  ContestAttackOptionKind,
  ContestAttackOptionSchema,
  ContestKind,
  ContestPurpose,
  ContestSkill,
} from '../../../../../gen/meurpg/play/v1/contest_types_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import { ContestClient } from '../../../../core/combat/contest-client';
import { ContestState } from '../../../../core/combat/contest-state';
import { FakeContestClient, contestView, textOf } from '../../../../core/combat/contest-testing';
import { type NpcGrappleData, NpcGrappleSheet } from './npc-grapple-sheet';

function setup(over: Partial<NpcGrappleData> = {}) {
  const api = new FakeContestClient();
  const e = encounter({
    combatants: [
      combatant({ id: 'b', label: 'Brisa', kind: CombatantKind.PLAYER }),
      combatant({ id: 't', label: 'Toren', kind: CombatantKind.PLAYER }),
      combatant({ id: 'hob', label: 'Hobgoblin' }),
      combatant({ id: 'cobra', label: 'Cobra constritora gigante' }),
      combatant({ id: 'dead', label: 'Goblin 9', defeated: true }),
    ],
  });
  api.encounterAnswer = encounter({ ...e, revision: 9 } as never);
  api.contest = contestView({ id: 'ct7' });
  const state = new CombatState();
  state.encounter.set(e);
  const contests = new ContestState();
  const close = vi.fn();
  const data: NpcGrappleData = {
    campaignId: 'camp',
    encounterId: 'enc',
    state,
    contests,
    diceMode: DiceMode.PLAYERS_CHOOSE,
    preference: DicePreference.APP,
    initiatorId: 'hob',
    attacks: [
      create(ContestAttackOptionSchema, {
        kind: ContestAttackOptionKind.GRAPPLE,
        rollOption: create(CheckOptionSchema, { modifier: 1, known: true }),
      }),
    ],
    ...over,
  };
  TestBed.configureTestingModule({
    providers: [
      { provide: ContestClient, useValue: api.as() },
      { provide: MAT_DIALOG_DATA, useValue: data },
      { provide: MatDialogRef, useValue: { close, disableClose: false } },
    ],
  });
  const fixture = TestBed.createComponent(NpcGrappleSheet);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const button = (name: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      textOf(b).includes(name),
    );
  const settle = async () => {
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  };
  const radio = (label: string) =>
    Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]')).find((r) =>
      textOf(r.closest('label')!).startsWith(label),
    )!;
  return { api, fixture, el, button, settle, close, state, contests, radio };
}

describe('NpcGrappleSheet, a creature grapples or shoves', () => {
  it('lists the creatures that attack and everyone else as the target, never the defeated', () => {
    const { el } = setup();
    const options = (id: string) =>
      Array.from(el.querySelectorAll<HTMLOptionElement>(`#${id} option`)).map((o) =>
        o.textContent?.trim(),
      );
    expect(options('npc-grapple-who')).toEqual(['Hobgoblin', 'Cobra constritora gigante']);
    expect(options('npc-grapple-target')).toEqual(['Brisa', 'Toren', 'Cobra constritora gigante']);
    expect(textOf(el)).toContain('Agarrar ou empurrar por um NPC');
    expect(textOf(el)).toContain('Teste de Força (Atletismo) pelo Hobgoblin.');
  });

  it('rolls Força (Atletismo) for the creature in the app and opens a contest', async () => {
    const { api, button, settle, close, state, contests } = setup();
    button('Rolar pelo Hobgoblin')!.click();
    await settle();
    expect(api.started).toHaveLength(1);
    expect(api.started[0].input).toEqual({
      campaignId: 'camp',
      encounterId: 'enc',
      initiatorId: 'hob',
      targetId: 'b',
      purpose: ContestPurpose.GRAPPLE,
      kind: ContestKind.CONTEST,
      skill: ContestSkill.ATHLETICS,
      die: { inApp: true },
    });
    expect(api.started[0].key).toEqual(expect.any(String));
    expect(state.encounter()?.revision).toBe(9);
    expect(contests.contest('ct7')).toBeTruthy();
    expect(close).toHaveBeenCalledWith({ contestId: 'ct7' });
  });

  it('sets the fixed escape DC of the creature: no roll, the DC is sent', async () => {
    const { api, el, button, settle, radio, fixture, close } = setup();
    radio('CD de escape').click();
    await settle();
    const text = textOf(el);
    expect(text).toContain('CD de escape (1 a 40)');
    expect(text).toContain(
      'Para escapar, faz um teste de Atletismo ou Acrobacia contra esta CD. Só você a vê.',
    );
    expect(button('Agarrar')!.disabled).toBe(true);
    const field = el.querySelector<HTMLInputElement>('#npc-grapple-dc')!;
    field.value = '16';
    field.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    button('Agarrar')!.click();
    await settle();
    expect(api.started[0].input).toMatchObject({
      kind: ContestKind.ESCAPE_DC,
      purpose: ContestPurpose.GRAPPLE,
      escapeDc: 16,
    });
    expect(api.started[0].input.die).toBeUndefined();
    expect(close).toHaveBeenCalled();
  });

  it('refuses a DC out of 1 to 40', async () => {
    const { el, button, settle, radio, fixture } = setup();
    radio('CD de escape').click();
    await settle();
    const field = el.querySelector<HTMLInputElement>('#npc-grapple-dc')!;
    for (const bad of ['0', '41', 'x']) {
      field.value = bad;
      field.dispatchEvent(new Event('input'));
      fixture.detectChanges();
      expect(button('Agarrar')!.disabled).toBe(true);
    }
  });

  it('a shove is always a contest and never has an escape DC', async () => {
    const { api, el, button, settle, radio } = setup();
    radio('CD de escape').click();
    await settle();
    radio('Empurrar').click();
    await settle();
    expect(el.querySelector('#npc-grapple-dc')).toBeNull();
    expect(textOf(el)).not.toContain('CD de escape');
    button('Rolar pelo Hobgoblin')!.click();
    await settle();
    expect(api.started[0].input).toMatchObject({
      purpose: ContestPurpose.SHOVE,
      kind: ContestKind.CONTEST,
    });
  });

  it("says the server's refusal and stays open", async () => {
    const { api, el, button, settle, close } = setup();
    api.error = new ConnectError('far', Code.FailedPrecondition);
    button('Rolar pelo Hobgoblin')!.click();
    await settle();
    expect(el.querySelector('[role="alert"]')).toBeTruthy();
    expect(close).not.toHaveBeenCalled();
  });
});

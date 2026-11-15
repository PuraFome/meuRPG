import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  CombatantState,
  GetTurnOptionsResponseSchema,
  ReactionKind,
  ResourceTargetSchema,
  ResourceTargetsSchema,
  TargetInReachSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  ActionEconomy,
  ActionOptionSchema,
  FeatureTargetWho,
  FeatureTriggerKind,
  TurnOptionsSchema,
  UseScope,
} from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { CombatClient } from '../../../../core/combat/combat-client';
import { fakeCombatClient } from '../../../../core/combat/combat-client-testing';
import { CombatState as State } from '../../../../core/combat/combat-state';
import { combatant, encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { FeatureWindowSheet, type FeatureWindowSheetData } from './feature-window-sheet';

const flat = (n: Element | null | undefined) =>
  n?.textContent?.replace(/\s+/g, ' ').replace(/ /g, ' ').trim() ?? '';

const target = (id: string, label: string, reason = '', feet = 5) =>
  create(ResourceTargetSchema, {
    target: create(TargetInReachSchema, {
      combatantId: id,
      label,
      state: CombatantState.UNHURT,
      distanceFt: feet,
    }),
    disabledReasonPt: reason,
  });

const window = reactionWindow({
  id: 'w1',
  kind: ReactionKind.FEATURE,
  reactorId: 'sal',
  reactorLabel: 'Sálvia',
  reactorIsPlayer: true,
  trigger: {
    actorId: 'z1',
    actorLabel: 'Zumbi 1',
    actionKey: 'feature:halo',
    actionNamePt: 'Halo de esporos',
    distanceFt: 5,
    featureTrigger: FeatureTriggerKind.ENEMY_MOVES_WITHIN,
  } as never,
} as never);

const halo = create(ActionOptionSchema, {
  enabled: true,
  viaUseFeature: true,
  useScope: UseScope.COMBAT,
  useTotal: 2,
  useLeft: 1,
  targetMin: 1,
  targetMax: 1,
  targetWho: FeatureTargetWho.ENEMY,
  targetRangeFt: 10,
  action: { key: 'feature:halo', namePt: 'Halo de esporos', economy: ActionEconomy.REACTION },
} as never);

describe('FeatureWindowSheet (#743, board 743-02)', () => {
  const answerReaction = vi.fn();
  const close = vi.fn();

  function setup(opts: { fail?: boolean; gone?: boolean; reactionUsed?: boolean } = {}) {
    const state = new State();
    state.apply(
      encounter({
        combatants: [
          combatant({ id: 'sal', label: 'Sálvia', mine: true, reactionUsed: !!opts.reactionUsed }),
        ],
        reactionWindows: opts.gone ? [] : [window],
      } as never),
    );
    const data: FeatureWindowSheetData = {
      campaignId: 'c',
      encounterId: 'enc',
      window,
      round: 2,
      load: async () => {
        if (opts.fail) {
          throw new Error('down');
        }
        return create(GetTurnOptionsResponseSchema, {
          options: create(TurnOptionsSchema, { featureActions: [halo] }),
          featureTargets: [
            create(ResourceTargetsSchema, {
              actionKey: 'feature:halo',
              targets: [
                target('z1', 'Zumbi 1', '', 5),
                target('z2', 'Zumbi 2', 'Fora de alcance: até 3 m', 15),
              ],
            }),
          ],
        });
      },
      state,
    };
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        { provide: MAT_DIALOG_DATA, useValue: data },
        { provide: MatDialogRef, useValue: { close, disableClose: false } },
        { provide: CombatClient, useValue: fakeCombatClient({ answerReaction }) },
      ],
    });
    const fixture = TestBed.createComponent(FeatureWindowSheet);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
    };
    const button = (text: string) =>
      Array.from(el.querySelectorAll('button')).find((b) => flat(b).startsWith(text))!;
    return { fixture, el, settle, button, state };
  }

  beforeEach(() => {
    answerReaction.mockReset();
    close.mockReset();
  });

  it('names the feature, the creature and its distance, and what the reaction costs', async () => {
    const { el, settle } = setup();
    await settle();
    expect(flat(el.querySelector('h2'))).toBe('Halo de esporos');
    expect(flat(el.querySelector('.frame__sub'))).toBe('Reação · Zumbi 1 · Rodada 2');
    expect(flat(el.querySelector('.what'))).toBe(
      'O Zumbi 1 chegou a 1,5 m de você. Usar o Halo de esporos?',
    );
    expect(flat(el.querySelector('.small'))).toContain('Gasta a sua reação');
    expect(el.querySelector('.frame__close')).toBeNull();
  });

  it('lists whom the feature takes with the creature chosen, and "Não usar" first with the focus mark', async () => {
    const { el, settle, button } = setup();
    await settle();
    expect(flat(el.querySelector('legend'))).toBe('Em quem · 1 inimigo até 3 m');
    expect(el.querySelectorAll('input[type=radio]').length).toBe(2);
    expect(
      (el.querySelector('input[type=radio]:checked') as HTMLInputElement | null)?.closest('label')
        ?.textContent,
    ).toContain('Zumbi 1');
    const buttons = Array.from(el.querySelectorAll('.pair button')).map((b) => flat(b));
    expect(buttons).toEqual(['Não usar', 'Usar no Zumbi 1']);
    expect(button('Não usar').hasAttribute('data-initial-focus')).toBe(true);
    expect(flat(el.querySelector('.foot__clock'))).toMatch(
      /^Se você não responder, passa sozinho em \d+ s\.$/,
    );
  });

  it('answers USE with the creature chosen and closes', async () => {
    answerReaction.mockResolvedValue({ encounter: encounter({ revision: 5 } as never) });
    const { settle, button, state } = setup();
    await settle();
    button('Usar no Zumbi 1').click();
    await settle();
    expect(answerReaction).toHaveBeenCalledWith(
      'c',
      'enc',
      'w1',
      { use: true, creatureIds: ['z1'] },
      expect.any(String),
    );
    expect(state.encounter()?.revision).toBe(5);
    expect(close).toHaveBeenCalled();
  });

  it('answers DECLINE with nothing chosen', async () => {
    answerReaction.mockResolvedValue({ encounter: encounter({ revision: 6 } as never) });
    const { settle, button } = setup();
    await settle();
    button('Não usar').click();
    await settle();
    expect(answerReaction).toHaveBeenCalledWith(
      'c',
      'enc',
      'w1',
      { use: false, creatureIds: [] },
      expect.any(String),
    );
    expect(close).toHaveBeenCalled();
  });

  it('still uses the feature on the creature when the list cannot be read, and says so', async () => {
    answerReaction.mockResolvedValue({ encounter: encounter() });
    const { el, settle, button } = setup({ fail: true });
    await settle();
    expect(flat(el.querySelector('.body .small:last-of-type'))).toContain(
      'Não deu para ler quem está ao alcance',
    );
    button('Usar no Zumbi 1').click();
    await settle();
    expect(answerReaction).toHaveBeenCalledWith(
      'c',
      'enc',
      'w1',
      { use: true, creatureIds: ['z1'] },
      expect.any(String),
    );
  });

  it("writes the server's refusal in a red band and stays open", async () => {
    answerReaction.mockRejectedValue(new ConnectError('não', Code.FailedPrecondition));
    const { el, settle, button } = setup();
    await settle();
    button('Usar no Zumbi 1').click();
    await settle();
    expect(el.querySelector('[role=alert]')).not.toBeNull();
    expect(close).not.toHaveBeenCalled();
    expect(button('Usar no Zumbi 1').hasAttribute('disabled')).toBe(false);
  });

  it('says the time ran out when the window closed with the reaction free, and only closes', async () => {
    const { el, settle, button } = setup({ gone: true });
    await settle();
    expect(flat(el.querySelector('.what'))).toBe(
      'Você não respondeu a tempo: o Zumbi 1 seguiu. A sua reação continua livre.',
    );
    expect(el.querySelector('.pair')).toBeNull();
    button('Fechar').click();
    expect(close).toHaveBeenCalled();
  });

  it('says the master used it for the player when the reaction is spent', async () => {
    const { el, settle } = setup({ gone: true, reactionUsed: true });
    await settle();
    expect(flat(el.querySelector('.what'))).toBe(
      'O mestre respondeu por você: a reação foi usada no Zumbi 1.',
    );
  });
});

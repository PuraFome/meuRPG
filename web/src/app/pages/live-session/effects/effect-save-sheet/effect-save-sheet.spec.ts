import { TestBed } from '@angular/core/testing';
import { MAT_BOTTOM_SHEET_DATA, MatBottomSheetRef } from '@angular/material/bottom-sheet';
import { MAT_DIALOG_DATA, MatDialogRef } from '@angular/material/dialog';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { ReactionKind } from '../../../../../gen/meurpg/play/v1/combat_pb';
import {
  EffectPhase,
  EffectSaveResultSchema,
  ExtraDieSchema,
} from '../../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../../core/combat/combat-state';
import { encounter, reactionWindow } from '../../../../core/combat/combat-testing';
import { EffectsClient } from '../../../../core/effects/effects-client';
import { EffectSaveSheet, type EffectSaveSheetData } from './effect-save-sheet';

const plain = (text: string | null | undefined) =>
  (text ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

function save(over: object = {}, window: object = {}) {
  return reactionWindow({
    id: 'w1',
    kind: ReactionKind.EFFECT_SAVE,
    reactorId: 'me',
    reactorLabel: 'Brisa',
    reactorIsPlayer: true,
    prompt: {
      case: 'effectSave',
      value: {
        effectId: 'e1',
        sourceNamePt: 'Imobilizar Pessoa',
        ability: 'wis',
        abilityNamePt: 'Sabedoria',
        phase: EffectPhase.END,
        modifier: 1,
        bonusKnown: true,
        mode: 'normal',
        modeSourcesPt: [],
        extraDice: [],
        autoFail: false,
        handedToMaster: false,
        textPt:
          'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você. Se falhar, ele continua.',
        ...over,
      },
    } as never,
    ...window,
  } as never);
}

const passed = create(EffectSaveResultSchema, {
  d20: 12,
  modifier: 1,
  total: 13,
  saved: true,
  effectEnded: true,
  textPt: 'Imobilizar Pessoa acabou. Você pode se mover e agir no seu próximo turno.',
});

describe('EffectSaveSheet', () => {
  const api = { rollEffectSave: vi.fn() };
  const closed: unknown[] = [];

  function setup(window = save(), over: Partial<EffectSaveSheetData> & { phone?: boolean } = {}) {
    const state = new CombatState();
    state.apply(encounter({ reactionWindows: [window] } as never));
    const data: EffectSaveSheetData = {
      campaignId: 'camp',
      encounterId: 'enc',
      window,
      round: 3,
      state,
      diceMode: DiceMode.PLAYERS_CHOOSE,
      preference: DicePreference.APP,
      ...over,
    };
    closed.length = 0;
    const close = (r?: unknown) => closed.push(r ?? 'closed');
    TestBed.configureTestingModule({
      providers: [
        { provide: EffectsClient, useValue: api },
        ...(over.phone
          ? [
              { provide: MAT_BOTTOM_SHEET_DATA, useValue: data },
              { provide: MatBottomSheetRef, useValue: { dismiss: close, disableClose: false } },
            ]
          : [
              { provide: MAT_DIALOG_DATA, useValue: data },
              { provide: MatDialogRef, useValue: { close, disableClose: false } },
            ]),
      ],
    });
    const fixture = TestBed.createComponent(EffectSaveSheet);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, state };
  }

  const button = (el: HTMLElement, label: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button')).find((b) =>
      plain(b.textContent).includes(label),
    )!;
  const flush = async (fixture: { detectChanges(): void; whenStable(): Promise<unknown> }) => {
    await fixture.whenStable();
    fixture.detectChanges();
  };
  const field = (el: HTMLElement, label: string) => {
    const l = Array.from(el.querySelectorAll('label')).find((x) =>
      plain(x.textContent).includes(label),
    )!;
    return el.querySelector<HTMLInputElement>(`#${l.getAttribute('for')}`)!;
  };
  const fill = (input: HTMLInputElement, value: string) => {
    input.value = value;
    input.dispatchEvent(new Event('input'));
  };

  beforeEach(() => {
    api.rollEffectSave.mockReset();
  });

  describe('the sheet of the end of the turn', () => {
    it('says the ability and what a pass does, the modifier and the mode, with no DC', () => {
      const { el } = setup();
      expect(plain(el.querySelector('h2')?.textContent)).toBe('Fim do seu turno');
      expect(plain(el.querySelector('.frame__sub')?.textContent)).toBe('Imobilizar Pessoa');
      const text = plain(el.textContent);
      expect(text).toContain(
        'Teste de resistência de Sabedoria. Se passar, o efeito acaba sobre você. Se falhar, ele continua.',
      );
      expect(el.querySelector('b')?.textContent).toBe('Sabedoria');
      expect(text).toContain('Seu modificador: +1.');
      expect(text).toContain('Você não tem vantagem nem desvantagem neste teste.');
      expect(text).not.toMatch(/\bCD\b/);
    });

    it('has the three answers in a column, the app first and in focus', () => {
      const { el } = setup();
      const app = button(el, 'Rolar no app');
      const typed = button(el, 'Digitar o resultado');
      const hand = button(el, 'Deixar o mestre rolar por mim');
      expect(app.hasAttribute('data-initial-focus')).toBe(true);
      for (const b of [app, typed, hand]) {
        expect(b.classList).toContain('btn');
        expect(b.closest('.stack')).not.toBeNull();
      }
      expect(
        el.querySelector('ol[aria-label="Passos do teste"] [aria-current="step"]'),
      ).not.toBeNull();
    });

    it.each([
      ['390 px, the bottom sheet of a phone', true],
      ['1280 px, the dialog of a desktop', false],
    ])('draws the same sheet at %s', (_name, phone) => {
      const { el } = setup(save(), { phone });
      expect(el.querySelector('.frame--phone') !== null).toBe(phone);
      expect(el.querySelector('.frame__handle') !== null).toBe(phone);
      expect(button(el, 'Rolar no app')).toBeTruthy();
      expect(el.querySelector('.stack')?.children).toHaveLength(3);
    });

    it('shows no app button when the campaign rolls physical dice only, and opens the typed fields', () => {
      const { el } = setup(save(), { diceMode: DiceMode.PHYSICAL });
      expect(
        Array.from(el.querySelectorAll('button')).some(
          (b) => plain(b.textContent) === 'Rolar no app',
        ),
      ).toBe(false);
      expect(field(el, 'Resultado do d20')).toBeTruthy();
    });
  });

  describe('rolling in the app', () => {
    it('sends the window with a key and shows what happened with "Fechar"', async () => {
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: passed,
      });
      const { fixture, el } = setup();
      button(el, 'Rolar no app').click();
      await flush(fixture);
      const [campaign, enc, windowId, how, key] = api.rollEffectSave.mock.calls[0];
      expect([campaign, enc, windowId, how]).toEqual(['camp', 'enc', 'w1', { kind: 'app' }]);
      expect(key).toMatch(/^[0-9a-f-]{36}$/);
      const text = plain(el.textContent);
      expect(text).toContain('Passou');
      expect(text).toContain('1d20 (12) + 1');
      expect(text).toContain('Imobilizar Pessoa acabou.');
      expect(el.querySelector('[role="status"]')).not.toBeNull();
      button(el, 'Fechar').click();
      expect(closed).toEqual(['closed']);
    });

    it('says "Falhou" and keeps the effect when the save fails', async () => {
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: create(EffectSaveResultSchema, {
          d20: 4,
          modifier: 1,
          total: 5,
          saved: false,
          textPt: 'Você continua Paralisada.',
        }),
      });
      const { fixture, el } = setup();
      button(el, 'Rolar no app').click();
      await flush(fixture);
      expect(plain(el.textContent)).toContain('Falhou');
      expect(plain(el.textContent)).toContain('Você continua Paralisada.');
    });

    it('keeps the key of a retry after a refusal and makes a new one for another way', async () => {
      api.rollEffectSave.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
      api.rollEffectSave.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
      const { fixture, el } = setup();
      button(el, 'Rolar no app').click();
      await flush(fixture);
      expect(el.querySelector('[role="alert"]')).not.toBeNull();
      button(el, 'Rolar no app').click();
      await flush(fixture);
      expect(api.rollEffectSave.mock.calls[0][4]).toBe(api.rollEffectSave.mock.calls[1][4]);
      button(el, 'Deixar o mestre rolar por mim').click();
      await flush(fixture);
      expect(api.rollEffectSave.mock.calls[2][4]).not.toBe(api.rollEffectSave.mock.calls[0][4]);
    });
  });

  describe('typing the physical dice', () => {
    it('asks the d20 and sends its face', async () => {
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: create(EffectSaveResultSchema, { ...passed, physical: true }),
      });
      const { fixture, el } = setup();
      button(el, 'Digitar o resultado').click();
      fixture.detectChanges();
      fill(field(el, 'Resultado do d20'), '12');
      fixture.detectChanges();
      button(el, 'Confirmar').click();
      await flush(fixture);
      expect(api.rollEffectSave.mock.calls[0][3]).toEqual({
        kind: 'typed',
        face: 12,
        extra: [],
      });
      expect(plain(el.textContent)).toContain('dado físico');
    });

    it('asks the d4 of Bênção before confirming, and sends it with the d20', async () => {
      const prompt = {
        extraDice: [create(ExtraDieSchema, { sourceNamePt: 'Bênção', faces: 4, sign: 1 })],
      };
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: passed,
      });
      const { fixture, el } = setup(save(prompt));
      expect(plain(el.textContent)).toContain('Bênção: soma 1d4 a este teste.');
      button(el, 'Digitar o resultado').click();
      fixture.detectChanges();
      const d4 = field(el, 'Resultado do d4 (Bênção)');
      expect(plain(el.textContent)).toContain(
        'Bênção soma 1d4 a esta jogada. Role um d4 além do d20.',
      );
      fill(field(el, 'Resultado do d20'), '12');
      fixture.detectChanges();
      button(el, 'Confirmar').click();
      await flush(fixture);
      // The d4 is missing: nothing was sent.
      expect(api.rollEffectSave).not.toHaveBeenCalled();
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('Digite o resultado do d4');
      fill(d4, '9');
      fixture.detectChanges();
      expect(d4.getAttribute('aria-invalid')).toBe('true');
      fill(d4, '3');
      fixture.detectChanges();
      button(el, 'Confirmar').click();
      await flush(fixture);
      expect(api.rollEffectSave.mock.calls[0][3]).toEqual({ kind: 'typed', face: 12, extra: [3] });
    });

    it('asks two d20 with advantage and sends the second one', async () => {
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: passed,
      });
      const { fixture, el } = setup(
        save({ mode: 'advantage', modeSourcesPt: ['Esquivando: vantagem'] }),
      );
      expect(plain(el.textContent)).toContain(
        'Você tem vantagem neste teste (Esquivando: vantagem).',
      );
      button(el, 'Digitar o resultado').click();
      fixture.detectChanges();
      fill(field(el, 'Primeiro d20'), '7');
      fill(field(el, 'Segundo d20'), '15');
      fixture.detectChanges();
      button(el, 'Confirmar').click();
      await flush(fixture);
      expect(api.rollEffectSave.mock.calls[0][3]).toEqual({
        kind: 'typed',
        face: 7,
        second: 15,
        extra: [],
      });
    });

    it('asks for the d4 a refusal says the roll takes, when the effect behind it is not listed', async () => {
      api.rollEffectSave.mockRejectedValueOnce(
        new ConnectError(
          'the roll takes 1 more die(s): type their faces in extra_die_faces',
          Code.InvalidArgument,
        ),
      );
      const { fixture, el } = setup();
      button(el, 'Digitar o resultado').click();
      fixture.detectChanges();
      fill(field(el, 'Resultado do d20'), '12');
      fixture.detectChanges();
      button(el, 'Confirmar').click();
      await flush(fixture);
      expect(el.querySelector('[role="alert"]')?.textContent).toContain('mais um d4');
      expect(field(el, 'Resultado do d4 (Outra fonte)')).toBeTruthy();
    });
  });

  describe('the other states', () => {
    it('has no d20 for a save that fails by itself: one "Continuar" and the app answers it', async () => {
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: create(EffectSaveResultSchema, { autoFail: true, saved: false }),
      });
      const { fixture, el } = setup(save({ autoFail: true }));
      expect(plain(el.querySelector('[data-testid="auto-fail"]')?.textContent)).toContain(
        'Falha automática.',
      );
      expect(
        Array.from(el.querySelectorAll('button')).some((b) =>
          plain(b.textContent).includes('Rolar no app'),
        ),
      ).toBe(false);
      button(el, 'Continuar').click();
      await flush(fixture);
      expect(plain(el.textContent)).toContain('Sem rolagem.');
      expect(plain(el.textContent)).toContain('Falhou');
    });

    it('closes when the roll was left to the master, with nothing to read', async () => {
      api.rollEffectSave.mockResolvedValue({
        encounter: encounter({ reactionWindows: [] } as never),
        result: undefined,
      });
      const { fixture, el } = setup();
      button(el, 'Deixar o mestre rolar por mim').click();
      await flush(fixture);
      expect(api.rollEffectSave.mock.calls[0][3]).toEqual({ kind: 'hand' });
      expect(closed).toEqual(['closed']);
    });

    it('says a roll left with the master is waiting for him', () => {
      const { el } = setup(save({ handedToMaster: true }));
      expect(plain(el.textContent)).toContain('Esperando o mestre: você deixou a rolagem com ele.');
      expect(
        Array.from(el.querySelectorAll('button')).some((b) =>
          plain(b.textContent).includes('Rolar no app'),
        ),
      ).toBe(false);
    });

    it('says so when the window is gone (the master answered, or the effect ended)', () => {
      const { fixture, el, state } = setup();
      state.apply(encounter({ reactionWindows: [] } as never));
      fixture.detectChanges();
      expect(plain(el.textContent)).toContain('Este teste já foi respondido');
      expect(el.querySelector('[role="status"]')).not.toBeNull();
    });

    it('closes with the X and with Esc', () => {
      const { fixture, el } = setup();
      el.querySelector<HTMLButtonElement>('.frame__close')!.click();
      expect(closed).toEqual(['closed']);
      closed.length = 0;
      el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      fixture.detectChanges();
      expect(closed).toEqual(['closed']);
    });
  });
});

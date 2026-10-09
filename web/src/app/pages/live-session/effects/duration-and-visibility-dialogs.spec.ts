import { Code, ConnectError } from '@connectrpc/connect';

import {
  EffectAudience,
  EffectDurationKind,
} from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { CombatState } from '../../../core/combat/combat-state';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { boardEffects } from '../../../core/effects/effects-testing';
import { DurationDialog, type DurationData } from './duration-dialog';
import { flat, isOff, openDialog } from './effects-dialogs-testing';
import { VisibilityDialog, type VisibilityData } from './visibility-dialog';

function stateOfCombat(): CombatState {
  const state = new CombatState();
  state.apply(
    encounter({
      id: 'enc',
      revision: 1,
      combatants: [
        combatant({ id: 'g2', label: 'Goblin 2' }),
        combatant({ id: 'orla', label: 'Orla' }),
      ],
    }),
  );
  return state;
}

describe('DurationDialog (W7-E board 4b)', () => {
  const changeDuration = vi.fn();
  let data: DurationData;

  beforeEach(() => {
    changeDuration.mockReset().mockResolvedValue(encounter({ id: 'enc', revision: 4 }));
    data = {
      campaignId: 'camp',
      encounterId: 'enc',
      state: stateOfCombat(),
      effect: { ...boardEffects().effects[1], roundsLeft: 9, targetIds: ['g2'] },
    };
  });

  afterEach(() => document.body.replaceChildren());

  it('names the effect and its target, and says how it ends today', () => {
    const { el } = openDialog(DurationDialog, data, { changeDuration });
    expect(flat(el.querySelector('h2'))).toBe('Mudar a duração de Imobilizar Pessoa em Goblin 2');
    const titles = Array.from(el.querySelectorAll('.choice__title')).map(flat);
    expect(titles).toEqual([
      'Mais rodadas',
      'Até o fim do turno de alguém',
      'Até o mestre encerrar',
    ]);
    expect(flat(el.querySelector('.choice__text'))).toBe(
      'Hoje resta 9. Acaba no turno de Orla, rodada 12.',
    );
    expect((el.querySelector('input[data-field="rounds"]') as HTMLInputElement).value).toBe('9');
    expect(flat(el.querySelector('app-field-note'))).toContain('De 1 a 600.');
  });

  it('marks the radio that fits the effect and draws one group', () => {
    const { el } = openDialog(DurationDialog, data, { changeDuration });
    const radios = Array.from(el.querySelectorAll('input[type="radio"]')) as HTMLInputElement[];
    expect(radios.map((r) => r.checked)).toEqual([true, false, false]);
    expect(new Set(radios.map((r) => r.name)).size).toBe(1);
    expect(el.querySelector('fieldset.choices legend')?.textContent).toContain(
      'Como o efeito acaba',
    );
  });

  it('saves a number of rounds from now', async () => {
    const { button, settle, type, close } = openDialog(DurationDialog, data, { changeDuration });
    await type('rounds', '12');
    button('Salvar')!.click();
    await settle();
    expect(changeDuration).toHaveBeenCalledWith(
      'camp',
      'enc',
      'hold-2',
      { kind: EffectDurationKind.ROUNDS, rounds: 12, anchorCombatantId: undefined },
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith(true);
    expect(data.state.encounter()?.revision).toBe(4);
  });

  it('saves "until the end of someone\'s turn" with the one picked, and "until the master ends"', async () => {
    const first = openDialog(DurationDialog, data, { changeDuration });
    await first.radio('Até o fim do turno de alguém');
    await first.pick('anchor', 'Orla');
    first.button('Salvar')!.click();
    await first.settle();
    expect(changeDuration.mock.calls[0][3]).toEqual({
      kind: EffectDurationKind.UNTIL_END_OF_TURN_OF,
      anchorCombatantId: 'orla',
    });
  });

  it('saves "until the master ends" with no end marked', async () => {
    const { radio, button, settle } = openDialog(DurationDialog, data, { changeDuration });
    await radio('Até o mestre encerrar');
    button('Salvar')!.click();
    await settle();
    expect(changeDuration.mock.calls[0][3]).toEqual({ kind: EffectDurationKind.UNTIL_DISMISSED });
  });

  it('refuses 0 and 601 rounds before asking the server', async () => {
    const { button, type } = openDialog(DurationDialog, data, { changeDuration });
    await type('rounds', '0');
    expect(isOff(button('Salvar')!)).toBe(true);
    await type('rounds', '601');
    expect(isOff(button('Salvar')!)).toBe(true);
    await type('rounds', '1');
    expect(isOff(button('Salvar')!)).toBe(false);
  });

  it('shows the refusal in words and keeps the key for a retry', async () => {
    changeDuration.mockRejectedValueOnce(new ConnectError('x', Code.NotFound));
    const { button, settle, el, close } = openDialog(DurationDialog, data, { changeDuration });
    button('Salvar')!.click();
    await settle();
    expect(flat(el.querySelector('[role="alert"]'))).toContain('não existe mais');
    expect(close).not.toHaveBeenCalled();
    button('Salvar')!.click();
    await settle();
    expect(changeDuration.mock.calls[1][4]).toBe(changeDuration.mock.calls[0][4]);
  });
});

describe('VisibilityDialog (W7-E board 4c)', () => {
  const setVisibility = vi.fn();
  let data: VisibilityData;

  beforeEach(() => {
    setVisibility.mockReset().mockResolvedValue(encounter({ id: 'enc', revision: 6 }));
    data = {
      campaignId: 'camp',
      encounterId: 'enc',
      state: stateOfCombat(),
      effect: { ...boardEffects().effects[1], playerLabel: 'Paralisado' },
    };
  });

  afterEach(() => document.body.replaceChildren());

  it('draws the card of the board: the switch, who sees, and the 30 characters label', () => {
    const { el } = openDialog(VisibilityDialog, data, { setVisibility });
    expect(flat(el.querySelector('h2'))).toBe(
      'O que os jogadores veem de Imobilizar Pessoa em Goblin 2',
    );
    const sw = el.querySelector('button[role="switch"]') as HTMLButtonElement;
    expect(sw.getAttribute('aria-checked')).toBe('true');
    expect(flat(el.querySelector('app-hidden-switch'))).toContain('Os jogadores veem este efeito');
    expect(flat(el.querySelector('app-hidden-switch'))).toContain(
      'Desligado, nada o revela: nem a espera, nem a fonte de vantagem, nem o registro.',
    );
    const titles = Array.from(el.querySelectorAll('.choice__title')).map(flat);
    expect(titles).toEqual(['Todos os jogadores', 'Só o dono do alvo']);
    const label = el.querySelector('input[maxlength="30"]') as HTMLInputElement;
    expect(label.value).toBe('Paralisado');
    expect(flat(el.querySelector('app-field-note'))).toBe(
      'Até 30 caracteres. Aparece só enquanto o efeito está visível. O app não confere o texto: não ponha números nele.',
    );
  });

  it('has nothing to save until something changes', () => {
    const { button } = openDialog(VisibilityDialog, data, { setVisibility });
    expect(isOff(button('Salvar')!)).toBe(true);
  });

  it('turns the visibility off and hides the rest of the card', async () => {
    const { el, button, settle } = openDialog(VisibilityDialog, data, { setVisibility });
    (el.querySelector('button[role="switch"]') as HTMLButtonElement).click();
    await settle();
    expect(el.querySelector('app-visibility-fields')).toBeNull();
    button('Salvar')!.click();
    await settle();
    expect(setVisibility).toHaveBeenCalledWith(
      'camp',
      'enc',
      'hold-2',
      { playerVisible: false, audience: EffectAudience.ALL, playerLabel: 'Paralisado' },
      expect.any(String),
    );
    expect(data.state.encounter()?.revision).toBe(6);
  });

  it('saves the other audience and a new label', async () => {
    const { button, settle, radio, el } = openDialog(VisibilityDialog, data, { setVisibility });
    await radio('Só o dono do alvo');
    const label = el.querySelector('input[maxlength="30"]') as HTMLInputElement;
    label.value = 'Preso';
    label.dispatchEvent(new Event('input'));
    await settle();
    button('Salvar')!.click();
    await settle();
    expect(setVisibility.mock.calls[0][3]).toEqual({
      playerVisible: true,
      audience: EffectAudience.OWNER,
      playerLabel: 'Preso',
    });
  });

  it('shows the refusal and stays open', async () => {
    setVisibility.mockRejectedValueOnce(new ConnectError('x', Code.PermissionDenied));
    const { el, button, settle, close } = openDialog(VisibilityDialog, data, { setVisibility });
    (el.querySelector('button[role="switch"]') as HTMLButtonElement).click();
    await settle();
    button('Salvar')!.click();
    await settle();
    expect(flat(el.querySelector('[role="alert"] p'))).toBe('Só o mestre faz isso.');
    expect(close).not.toHaveBeenCalled();
  });
});

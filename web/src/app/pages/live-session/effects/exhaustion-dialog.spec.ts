import { Code, ConnectError } from '@connectrpc/connect';

import { ExhaustionLowerReason } from '../../../../gen/meurpg/play/v1/lasting_effects_service_pb';
import { CombatState } from '../../../core/combat/combat-state';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { flat, isOff, openDialog } from './effects-dialogs-testing';
import { ExhaustionDialog, type ExhaustionData } from './exhaustion-dialog';

describe('ExhaustionDialog (W7-E board 9)', () => {
  const setExhaustion = vi.fn();
  const lowerExhaustion = vi.fn();
  let data: ExhaustionData;
  let state: CombatState;

  beforeEach(() => {
    setExhaustion.mockReset().mockResolvedValue({ level: 4, hitPointsMax: 20 });
    lowerExhaustion.mockReset().mockResolvedValue({ level: 3, hitPointsMax: 20 });
    state = new CombatState();
    state.apply(
      encounter({
        id: 'enc',
        revision: 1,
        combatants: [
          combatant({ id: 'toren', label: 'Toren', exhaustionLevel: 2 }),
          combatant({ id: 'orla', label: 'Orla', exhaustionLevel: 0 }),
        ],
      }),
    );
    data = {
      campaignId: 'camp',
      state,
      selected: 'toren',
      options: [
        {
          key: 'toren',
          label: 'Toren',
          level: 2,
          subject: { combatantId: 'toren', encounterId: 'enc' },
        },
        {
          key: 'orla',
          label: 'Orla',
          level: 0,
          subject: { combatantId: 'orla', encounterId: 'enc' },
        },
      ],
    };
  });

  afterEach(() => document.body.replaceChildren());

  it('shows the seven radios of the board, the marked one is the level now', () => {
    const { el } = openDialog(ExhaustionDialog, data, { setExhaustion, lowerExhaustion });
    expect(flat(el.querySelector('h2'))).toBe('Exaustão de Toren');
    const radios = Array.from(el.querySelectorAll('input[type="radio"]')) as HTMLInputElement[];
    expect(radios.map((r) => r.checked)).toEqual([false, false, true, false, false, false, false]);
    expect(Array.from(el.querySelectorAll('.choice__title')).map(flat)).toEqual([
      'Sem exaustão',
      'Nível 1',
      'Nível 2',
      'Nível 3',
      'Nível 4',
      'Nível 5',
      'Nível 6',
    ]);
    expect(flat(el.querySelectorAll('.choice__text')[3])).toBe(
      'PV máximos pela metade (e os de baixo)',
    );
    expect(flat(el.querySelector('.note'))).toContain(
      'baixar o nível não cura. O nível 6 é a morte.',
    );
  });

  it('has Salvar, Baixar 1 nível and Cancelar, and Salvar waits for a change', () => {
    const { button } = openDialog(ExhaustionDialog, data, { setExhaustion, lowerExhaustion });
    expect(isOff(button('Salvar')!)).toBe(true);
    expect(isOff(button('Baixar 1 nível')!)).toBe(false);
    expect(button('Cancelar')).toBeTruthy();
  });

  it('saves the level with the level the screen showed, and tells who changed', async () => {
    const { button, settle, radio, close } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    await radio('Nível 4');
    button('Salvar')!.click();
    await settle();
    expect(setExhaustion).toHaveBeenCalledWith(
      'camp',
      { combatantId: 'toren', encounterId: 'enc' },
      { level: 4, expectedLevel: 2, confirmDeath: false },
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith({ key: 'toren', level: 4 });
  });

  it('lowers one level, never more, with the level it showed', async () => {
    const { button, settle, close } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    button('Baixar 1 nível')!.click();
    await settle();
    expect(lowerExhaustion).toHaveBeenCalledWith(
      'camp',
      { combatantId: 'toren', encounterId: 'enc' },
      { by: 1, expectedLevel: 2, reason: ExhaustionLowerReason.MASTER },
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith({ key: 'toren', level: 3 });
  });

  it('cannot lower a creature with no exhaustion', async () => {
    const { button, pick, el } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    await pick('who', 'Orla');
    expect(flat(el.querySelector('h2'))).toBe('Exaustão de Orla');
    expect(isOff(button('Baixar 1 nível')!)).toBe(true);
    const radios = Array.from(el.querySelectorAll('input[type="radio"]')) as HTMLInputElement[];
    expect(radios[0].checked).toBe(true);
  });

  it('follows the level the combat has now, and sends that as the expected one', async () => {
    const { button, settle, radio } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    state.apply(
      encounter({
        id: 'enc',
        revision: 2,
        combatants: [
          combatant({ id: 'toren', label: 'Toren', exhaustionLevel: 3 }),
          combatant({ id: 'orla', label: 'Orla' }),
        ],
      }),
    );
    await settle();
    await radio('Nível 5');
    button('Salvar')!.click();
    await settle();
    expect(setExhaustion.mock.calls[0][2]).toEqual({
      level: 5,
      expectedLevel: 3,
      confirmDeath: false,
    });
  });

  it('turns level 6 into the question of death, with the focus on "Cancelar"', async () => {
    const { el, button, settle, radio } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    await radio('Nível 6');
    button('Salvar')!.click();
    await settle();
    expect(setExhaustion).not.toHaveBeenCalled();
    expect(flat(el.querySelector('h2'))).toBe('Exaustão nível 6: Toren morre?');
    expect(flat(el.querySelector('.ask__text'))).toBe(
      'O nível 6 é a morte. Toren vai para a confirmação de morte, como o terceiro teste contra a morte falhado. Isto não se desfaz.',
    );
    expect(button('Confirmar a morte')!.classList.contains('pair__btn--danger')).toBe(true);
    expect(document.activeElement).toBe(button('Cancelar'));
    expect(button('Salvar')).toBeUndefined();
  });

  it('goes back to the levels from the question, and kills nothing', async () => {
    const { el, button, settle, radio } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    await radio('Nível 6');
    button('Salvar')!.click();
    await settle();
    button('Cancelar')!.click();
    await settle();
    expect(flat(el.querySelector('h2'))).toBe('Exaustão de Toren');
    expect(setExhaustion).not.toHaveBeenCalled();
  });

  it('confirms the death with confirm_death set', async () => {
    setExhaustion.mockResolvedValue({ level: 6, hitPointsMax: 20 });
    const { button, settle, radio, close } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    await radio('Nível 6');
    button('Salvar')!.click();
    await settle();
    button('Confirmar a morte')!.click();
    await settle();
    expect(setExhaustion).toHaveBeenCalledWith(
      'camp',
      { combatantId: 'toren', encounterId: 'enc' },
      { level: 6, expectedLevel: 2, confirmDeath: true },
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith({ key: 'toren', level: 6 });
  });

  it('says the level changed meanwhile when the server answers aborted', async () => {
    setExhaustion.mockRejectedValueOnce(new ConnectError('x', Code.Aborted));
    const { el, button, settle, radio, close } = openDialog(ExhaustionDialog, data, {
      setExhaustion,
      lowerExhaustion,
    });
    await radio('Nível 3');
    button('Salvar')!.click();
    await settle();
    expect(flat(el.querySelector('[role="alert"]'))).toContain('O nível de exaustão mudou');
    expect(close).not.toHaveBeenCalled();
  });

  it('works for a character outside a combat, with no encounter', async () => {
    const outside: ExhaustionData = {
      campaignId: 'camp',
      state: null,
      options: [{ key: 'ch1', label: 'Ragna', level: 1, subject: { characterId: 'ch1' } }],
    };
    setExhaustion.mockResolvedValue({ level: 2, hitPointsMax: 40 });
    const { button, settle, radio, close, el } = openDialog(ExhaustionDialog, outside, {
      setExhaustion,
      lowerExhaustion,
    });
    expect(el.querySelector('app-select-field')).toBeNull();
    await radio('Nível 2');
    button('Salvar')!.click();
    await settle();
    expect(setExhaustion).toHaveBeenCalledWith(
      'camp',
      { characterId: 'ch1' },
      { level: 2, expectedLevel: 1, confirmDeath: false },
      expect.any(String),
    );
    expect(close).toHaveBeenCalledWith({ key: 'ch1', level: 2 });
  });
});

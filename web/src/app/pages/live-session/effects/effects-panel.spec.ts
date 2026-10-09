import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import { EffectEndScope } from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { ListLastingEffectsResponseSchema } from '../../../../gen/meurpg/play/v1/lasting_effects_service_pb';
import { CombatState } from '../../../core/combat/combat-state';
import { combatant, encounter } from '../../../core/combat/combat-testing';
import { EffectsClient } from '../../../core/effects/effects-client';
import { boardEffects } from '../../../core/effects/effects-testing';
import { EffectsPanel } from './effects-panel';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

function party() {
  return [
    combatant({ id: 'nael', label: 'Nael' }),
    combatant({ id: 'toren', label: 'Toren' }),
    combatant({ id: 'brisa', label: 'Brisa' }),
    combatant({ id: 'ragna', label: 'Ragna' }),
  ];
}

describe('EffectsPanel (W7-E board 4)', () => {
  const list = vi.fn();
  const end = vi.fn();
  const removeTarget = vi.fn();
  let state: CombatState;

  beforeEach(() => {
    list.mockReset().mockResolvedValue(boardEffects());
    end.mockReset().mockResolvedValue({ encounter: undefined, ended: 1, createdEffectIds: [] });
    removeTarget.mockReset().mockResolvedValue(undefined);
    state = new CombatState();
    state.apply(
      encounter({ id: 'enc', revision: 1, currentCombatantId: 'nael', combatants: party() }),
    );
    TestBed.configureTestingModule({
      providers: [{ provide: EffectsClient, useValue: { list, end, removeTarget } }],
    });
  });

  afterEach(() => document.body.replaceChildren());

  async function render() {
    const fixture = TestBed.createComponent(EffectsPanel);
    document.body.appendChild(fixture.nativeElement);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('state', state);
    fixture.detectChanges();
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
      await fixture.whenStable();
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const row = (name: string) =>
      Array.from(el.querySelectorAll('li.fx__row')).find(
        (r) => flat(r.querySelector('h3')) === name,
      )! as HTMLElement;
    return { fixture, el, settle, row };
  }

  it('reads the effects of the combat on open and lists each one as a row', async () => {
    const { el } = await render();
    expect(list).toHaveBeenCalledWith('camp', 'enc');
    expect(flat(el.querySelector('h2'))).toBe('Efeitos em jogo');
    expect(flat(el.querySelector('.fx__count'))).toBe('5 efeitos · rodada 3 · vez de Nael');
    const names = Array.from(el.querySelectorAll('li.fx__row h3')).map(flat);
    expect(names).toEqual([
      'Imobilizar Pessoa',
      'Imobilizar Pessoa',
      'Bênção',
      'Esquivando',
      'Derrubado',
    ]);
    expect(el.querySelectorAll('.fx__pill')).toHaveLength(3);
  });

  it('shows what the table of the board has in each column', async () => {
    const { row } = await render();
    const hold = row('Imobilizar Pessoa');
    expect(flat(hold.querySelector('.fx__who'))).toBe('Em quem: Brisa');
    expect(flat(hold.querySelector('.fx__origin'))).toBe('Origem: De Fanático do culto');
    const lines = Array.from(hold.querySelectorAll('.fx__line')).map(flat);
    expect(lines[0]).toContain('Resta 9 rodadas: acaba no turno de Orla, rodada 12.');
    expect(lines[1]).toContain('Teste de Sabedoria, CD 11, no fim do turno de Brisa.');
    expect(flat(hold.querySelector('.fx__seedesk'))).toBe('Jogadores veem: Sim: Paralisada');
    const bless = row('Bênção');
    expect(flat(bless.querySelector('.fx__who'))).toBe('Em quem: Toren, Brisa, Ragna (3 alvos)');
    expect(flat(bless.querySelector('.fx__tags'))).toBe('+1d4 em ataques e resistências');
  });

  it('draws the phone card: the subtitle, the pill that opens the visibility and full-width buttons', async () => {
    const { row } = await render();
    const hold = row('Imobilizar Pessoa');
    expect(flat(hold.querySelector('.fx__sub'))).toBe('Em Brisa · de Fanático do culto');
    expect(flat(hold.querySelector('.fx__seephone'))).toBe('Jogadores veem: Paralisada');
    const buttons = Array.from(hold.querySelectorAll('.fx__actions button'));
    expect(
      buttons.every((b) => b.classList.contains('fx__btn') || b.classList.contains('fx__link')),
    ).toBe(true);
    expect(buttons.map(flat)).toEqual(['Encerrar', 'Mudar a duração']);
  });

  it('names each button after its effect, for the keyboard and the screen reader', async () => {
    const { row, el } = await render();
    const bless = row('Bênção');
    expect(bless.getAttribute('aria-labelledby')).toBe('fx-n-bless');
    expect(el.querySelector('ul.fx__rows')?.getAttribute('aria-label')).toBe('Efeitos em jogo');
    expect(el.querySelector('.fx__cols')?.getAttribute('aria-hidden')).toBe('true');
    const buttons = Array.from(bless.querySelectorAll('.fx__actions button'));
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual([
      'Encerrar Bênção em Toren, Brisa, Ragna (3 alvos)',
      'Mudar a duração de Bênção em Toren, Brisa, Ragna (3 alvos)',
      'Tirar de um alvo, Bênção',
    ]);
    expect(flat(buttons[0])).toBe('Encerrar Bênção');
    expect(flat(buttons[2])).toBe('Tirar de um alvo');
  });

  it('reads the clock of the turns, in order, from the turn that runs', async () => {
    const { el } = await render();
    expect(flat(el.querySelector('.fx__clocktitle'))).toBe(
      'O relógio dos turnos, a partir da vez de Nael',
    );
    const titles = Array.from(el.querySelectorAll('.fx__ticktitle')).map(flat);
    expect(titles).toEqual([
      'Fim do turno de Goblin 2 (rodada 3)',
      'Fim do turno de Brisa (rodada 4)',
      'Começo do turno de Toren (rodada 4)',
      'Turno de Tavo (rodada 11)',
    ]);
    const first = el.querySelector('.fx__tick');
    expect(flat(first?.querySelector('.fx__ticktext'))).toBe(
      'Teste de Sabedoria (CD 14) contra Imobilizar Pessoa, de Orla.',
    );
    expect(flat(first?.querySelector('mat-icon'))).toBe('casino');
  });

  it('ends an effect that is not a concentration without a question', async () => {
    const { row, settle, el } = await render();
    (row('Derrubado').querySelector('.fx__actions button') as HTMLButtonElement).click();
    await settle();
    expect(end).toHaveBeenCalledWith(
      'camp',
      'enc',
      'prone',
      EffectEndScope.THIS,
      expect.any(String),
    );
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(flat(el.querySelector('[role="status"]'))).toBe('Efeito encerrado: Derrubado.');
    expect(list).toHaveBeenCalledTimes(2);
  });

  it('asks before ending a concentration, in danger-outline with the focus on "Cancelar"', async () => {
    const { row, settle, el } = await render();
    (row('Bênção').querySelector('[data-end]') as HTMLButtonElement).click();
    await settle();
    const ask = el.querySelector('[role="alertdialog"]') as HTMLElement;
    expect(ask).toBeTruthy();
    expect(flat(ask.querySelector('h4'))).toBe('Encerrar Bênção de Tavo?');
    expect(flat(ask.querySelector('p'))).toBe(
      'A concentração de Tavo acaba e Bênção sai de Toren, Brisa e Ragna. Isto não se desfaz.',
    );
    expect(ask.getAttribute('aria-labelledby')).toBe('fx-ask-t-bless');
    expect(ask.getAttribute('aria-describedby')).toBe('fx-ask-d-bless');
    expect(ask.querySelector('.fx__btn--danger')).toBeTruthy();
    expect(document.activeElement).toBe(ask.querySelector('[data-initial-focus]'));
    expect(flat(document.activeElement)).toBe('Cancelar');
    expect(end).not.toHaveBeenCalled();
  });

  it('gives the focus back to "Encerrar" when the question is cancelled, and Esc cancels it', async () => {
    const { row, settle, el } = await render();
    const endButton = row('Bênção').querySelector('[data-end]') as HTMLButtonElement;
    endButton.click();
    await settle();
    (el.querySelector('[data-initial-focus]') as HTMLButtonElement).click();
    await settle();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(document.activeElement).toBe(endButton);

    endButton.click();
    await settle();
    el.querySelector('[role="alertdialog"]')!.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    );
    await settle();
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(end).not.toHaveBeenCalled();
  });

  it('ends the whole concentration once confirmed', async () => {
    const { row, settle, el } = await render();
    (row('Bênção').querySelector('[data-end]') as HTMLButtonElement).click();
    await settle();
    const confirm = el.querySelector('.fx__btn--danger') as HTMLButtonElement;
    expect(flat(confirm)).toBe('Encerrar Bênção');
    confirm.click();
    await settle();
    expect(end).toHaveBeenCalledWith(
      'camp',
      'enc',
      'bless',
      EffectEndScope.CONCENTRATION_GROUP,
      expect.any(String),
    );
    expect(el.querySelector('[role="alertdialog"]')).toBeNull();
    expect(document.activeElement).toBe(el.querySelector('[data-effects-title]'));
  });

  it('takes one target off an effect that has several', async () => {
    const { row, settle, el } = await render();
    (row('Bênção').querySelector('.fx__link') as HTMLButtonElement).click();
    await settle();
    const group = el.querySelector('.fx__remove') as HTMLElement;
    expect(group.getAttribute('role')).toBe('group');
    expect(Array.from(group.querySelectorAll('button')).map(flat)).toEqual([
      'Tirar Toren',
      'Tirar Brisa',
      'Tirar Ragna',
    ]);
    (group.querySelectorAll('button')[1] as HTMLButtonElement).click();
    await settle();
    expect(removeTarget).toHaveBeenCalledWith('camp', 'enc', 'bless', 'brisa', expect.any(String));
    expect(el.querySelector('.fx__remove')).toBeNull();
  });

  it('keeps the same key for a retry after a failure and a new one after it worked', async () => {
    end.mockRejectedValueOnce(new ConnectError('lost', Code.Unavailable));
    const { row, settle } = await render();
    const button = () => row('Derrubado').querySelector('.fx__actions button') as HTMLButtonElement;
    button().click();
    await settle();
    button().click();
    await settle();
    button().click();
    await settle();
    const keys = end.mock.calls.map((c) => c[4] as string);
    expect(keys[1]).toBe(keys[0]);
    expect(keys[2]).not.toBe(keys[0]);
  });

  it('says what went wrong in words and keeps the row', async () => {
    end.mockRejectedValueOnce(new ConnectError('x', Code.PermissionDenied));
    const { row, settle, el } = await render();
    (row('Derrubado').querySelector('.fx__actions button') as HTMLButtonElement).click();
    await settle();
    expect(flat(el.querySelector('.mr-notice--danger[role="alert"]'))).toContain(
      'Só o mestre faz isso.',
    );
    expect(row('Derrubado')).toBeTruthy();
  });

  it('reads again when the combat changes or the log has a new line', async () => {
    const { settle } = await render();
    expect(list).toHaveBeenCalledTimes(1);
    state.touchLog();
    await settle();
    expect(list).toHaveBeenCalledTimes(2);
    state.apply(
      encounter({ id: 'enc', revision: 2, currentCombatantId: 'nael', combatants: party() }),
    );
    await settle();
    expect(list).toHaveBeenCalledTimes(3);
  });

  it('never lets an older answer replace a newer one', async () => {
    let first!: (v: unknown) => void;
    list.mockReset();
    list.mockReturnValueOnce(new Promise((resolve) => (first = resolve)));
    list.mockResolvedValueOnce(
      create(ListLastingEffectsResponseSchema, {
        round: 5,
        effects: boardEffects().effects.slice(0, 1),
      }),
    );
    const { settle, el } = await render();
    state.touchLog();
    await settle();
    first(boardEffects());
    await settle();
    expect(el.querySelectorAll('li.fx__row')).toHaveLength(1);
  });

  it('says there is nothing in play, and still offers "Adicionar efeito"', async () => {
    list.mockResolvedValue(create(ListLastingEffectsResponseSchema, { round: 1 }));
    const { el } = await render();
    expect(flat(el.querySelector('.fx__empty'))).toBe('Nenhum efeito em jogo agora.');
    expect(flat(el.querySelector('[data-testid="add-effect"]'))).toContain('Adicionar efeito');
    expect(flat(el.querySelector('.fx__count'))).toBe('0 efeitos · rodada 1');
  });

  it('says when the effects could not be read', async () => {
    list.mockRejectedValue(new ConnectError('x', Code.Unavailable));
    const { el } = await render();
    expect(flat(el.querySelector('[role="alert"]'))).toContain('o servidor não respondeu');
  });

  it('opens "Adicionar efeito" as a dialog with the catalog the server sent', async () => {
    const { el, settle } = await render();
    const open = vi.spyOn(TestBed.inject(MatDialog), 'open');
    (el.querySelector('[data-testid="add-effect"]') as HTMLButtonElement).click();
    await settle();
    expect(open).toHaveBeenCalledTimes(1);
    const config = open.mock.calls[0][1] as {
      data: { catalog: { key: string }[]; encounterId: string };
    };
    expect(config.data.encounterId).toBe('enc');
    expect(config.data.catalog.map((c) => c.key)).toEqual([
      'spell:bless',
      'spell:haste',
      'condition:prone',
    ]);
    document.querySelectorAll('.cdk-overlay-container *').forEach(() => undefined);
    TestBed.inject(MatDialog).closeAll();
  });
});

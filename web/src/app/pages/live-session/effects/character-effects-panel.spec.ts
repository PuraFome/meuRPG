import { TestBed } from '@angular/core/testing';
import { MatDialog } from '@angular/material/dialog';
import { Code, ConnectError } from '@connectrpc/connect';

import { EffectAudience, EffectEndScope } from '../../../../gen/meurpg/play/v1/lasting_effects_pb';
import { EffectsClient } from '../../../core/effects/effects-client';
import { catalogEffect, characterEffect } from '../../../core/effects/effects-testing';
import { pensantusVitals } from '../../../core/play/live-testing';
import { CharacterEffectsPanel } from './character-effects-panel';
import { isOff } from './effects-dialogs-testing';

const flat = (n: Element | null | undefined) => n?.textContent?.replace(/\s+/g, ' ').trim();

describe('CharacterEffectsPanel (outside a combat)', () => {
  const listCharacterEffects = vi.fn();
  const end = vi.fn();
  const advanceTime = vi.fn();
  const catalog = vi.fn();

  beforeEach(() => {
    listCharacterEffects.mockReset().mockResolvedValue([
      characterEffect({ id: 'fx1' }),
      characterEffect({
        id: 'fx2',
        characterId: 'ch2',
        characterName: 'Ragna',
        sourceNamePt: 'Bênção',
        durationTextPt: 'dura enquanto a conjuradora se concentrar',
        concentration: true,
        tagsPt: ['+1d4 em ataques e resistências'],
        playerVisible: false,
        audience: EffectAudience.OWNER,
      }),
    ]);
    end.mockReset().mockResolvedValue({ ended: 1, createdEffectIds: [] });
    advanceTime.mockReset().mockResolvedValue(2);
    catalog
      .mockReset()
      .mockResolvedValue([catalogEffect({ key: 'spell:bless', namePt: 'Bênção' })]);
    TestBed.configureTestingModule({
      providers: [
        { provide: EffectsClient, useValue: { listCharacterEffects, end, advanceTime, catalog } },
      ],
    });
  });

  afterEach(() => document.body.replaceChildren());

  async function render() {
    const fixture = TestBed.createComponent(CharacterEffectsPanel);
    document.body.appendChild(fixture.nativeElement);
    fixture.componentRef.setInput('campaignId', 'camp');
    fixture.componentRef.setInput('party', [
      pensantusVitals({ characterId: 'ch1', name: 'Toren', exhaustionLevel: 2 }),
      pensantusVitals({ characterId: 'ch2', name: 'Ragna' }),
    ]);
    fixture.detectChanges();
    const settle = async () => {
      await fixture.whenStable();
      fixture.detectChanges();
      await fixture.whenStable();
    };
    await settle();
    const el = fixture.nativeElement as HTMLElement;
    const card = (name: string) =>
      Array.from(el.querySelectorAll('li.cx__row')).find((c) =>
        flat(c.querySelector('h3'))?.startsWith(name),
      ) as HTMLElement;
    return { fixture, el, settle, card };
  }

  it('lists the effects on the characters, with what the players see', async () => {
    const { el, card } = await render();
    expect(listCharacterEffects).toHaveBeenCalledWith('camp');
    expect(flat(el.querySelector('h2'))).toContain('Efeitos em jogo');
    expect(flat(el.querySelector('.cx__count'))).toBe('2 · fora do combate');
    const first = card('Passos Largos');
    expect(flat(first.querySelector('h3'))).toContain('Toren');
    expect(flat(first.querySelector('.cx__sub'))).toContain('dura 1 hora');
    expect(flat(first)).toContain('Os jogadores veem: todos');
    const second = card('Bênção');
    expect(flat(second.querySelector('.cx__pill'))).toContain('Concentração');
    expect(flat(second)).toContain('+1d4 em ataques e resistências');
    expect(flat(second)).toContain('Os jogadores não veem');
  });

  it('ends an effect on a character with an empty encounter id', async () => {
    const { card, settle, el } = await render();
    (card('Passos Largos').querySelector('[data-end]') as HTMLButtonElement).click();
    await settle();
    expect(end).toHaveBeenCalledWith('camp', '', 'fx1', EffectEndScope.THIS, expect.any(String));
    expect(flat(el.querySelector('[role="status"]'))).toBe(
      'Efeito encerrado: Passos Largos em Toren.',
    );
    expect(listCharacterEffects).toHaveBeenCalledTimes(2);
  });

  it('asks before ending a concentration, in danger-outline with the focus on "Cancelar"', async () => {
    const { card, settle, el } = await render();
    const endButton = card('Bênção').querySelector('[data-end]') as HTMLButtonElement;
    endButton.click();
    await settle();
    const ask = el.querySelector('[role="alertdialog"]') as HTMLElement;
    expect(flat(ask.querySelector('h4'))).toBe('Encerrar Bênção em Ragna?');
    expect(ask.querySelector('.cx__btn--danger')).toBeTruthy();
    expect(document.activeElement).toBe(ask.querySelector('[data-initial-focus]'));
    expect(end).not.toHaveBeenCalled();
    (ask.querySelector('[data-initial-focus]') as HTMLButtonElement).click();
    await settle();
    expect(document.activeElement).toBe(endButton);
    endButton.click();
    await settle();
    (el.querySelector('.cx__btn--danger') as HTMLButtonElement).click();
    await settle();
    expect(end).toHaveBeenCalledWith(
      'camp',
      '',
      'fx2',
      EffectEndScope.CONCENTRATION_GROUP,
      expect.any(String),
    );
  });

  it('reads again when the page says something changed', async () => {
    const { fixture, settle } = await render();
    fixture.componentRef.setInput('tick', 1);
    fixture.detectChanges();
    await settle();
    expect(listCharacterEffects).toHaveBeenCalledTimes(2);
  });

  it('says there is nothing, and still offers the time', async () => {
    listCharacterEffects.mockResolvedValue([]);
    const { el } = await render();
    expect(flat(el.querySelector('.cx__empty'))).toBe('Nenhum efeito nos personagens agora.');
    expect(el.querySelector('[data-testid="pass-time"]')).toBeTruthy();
  });

  it('shows the refusal of an end and keeps the effect', async () => {
    end.mockRejectedValueOnce(new ConnectError('x', Code.NotFound));
    const { card, settle, el } = await render();
    (card('Passos Largos').querySelector('[data-end]') as HTMLButtonElement).click();
    await settle();
    expect(flat(el.querySelector('[role="alert"]'))).toContain('não existe mais');
    expect(card('Passos Largos')).toBeTruthy();
  });

  describe('"Passar o tempo"', () => {
    it('offers the four buttons of the board, equal and outlined, and no filled button', async () => {
      const { el } = await render();
      const time = el.querySelector('[data-testid="pass-time"]') as HTMLElement;
      expect(flat(time.querySelector('.cx__timelabel'))).toContain('Passar o tempo');
      expect(Array.from(time.querySelectorAll('.cx__preset')).map(flat)).toEqual([
        '1 min',
        '10 min',
        '1 hora',
        '8 horas',
      ]);
      expect(
        Array.from(time.querySelectorAll('.cx__preset')).map((b) => b.getAttribute('aria-label')),
      ).toEqual(['Passar 1 minuto', 'Passar 10 minutos', 'Passar 1 hora', 'Passar 8 horas']);
      expect(el.querySelector('.mat-mdc-unelevated-button')).toBeNull();
      expect(el.querySelector('input[data-field="seconds"]')).toBeNull();
    });

    it('moves the time a button says at once, and tells how many effects ended', async () => {
      const { el, settle } = await render();
      const time = el.querySelector('[data-testid="pass-time"]') as HTMLElement;
      (time.querySelectorAll('.cx__preset')[1] as HTMLButtonElement).click();
      await settle();
      expect(advanceTime).toHaveBeenCalledWith('camp', 600, expect.any(String));
      expect(flat(el.querySelector('.cx__done'))).toBe('Passou 10 minutos. 2 efeitos acabaram.');
      expect(listCharacterEffects).toHaveBeenCalledTimes(2);
    });

    it('takes any number of seconds from 1 to 86,400 under "Outro tempo", and refuses the rest', async () => {
      const { el, settle } = await render();
      (el.querySelector('.cx__other') as HTMLButtonElement).click();
      await settle();
      const input = el.querySelector('input[data-field="seconds"]') as HTMLInputElement;
      const go = el.querySelector('.cx__go') as HTMLButtonElement;
      const typeSeconds = async (v: string) => {
        input.value = v;
        input.dispatchEvent(new Event('input'));
        await settle();
      };
      await typeSeconds('0');
      expect(isOff(go)).toBe(true);
      expect(flat(el.querySelector('app-field-note'))).toContain('Digite de 1 a 86.400 segundos.');
      await typeSeconds('86401');
      expect(isOff(go)).toBe(true);
      await typeSeconds('86400');
      expect(isOff(go)).toBe(false);
      go.click();
      await settle();
      expect(advanceTime).toHaveBeenCalledWith('camp', 86400, expect.any(String));
    });

    it('says the time did not pass when the server refuses, and retries with the same key', async () => {
      advanceTime.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
      const { el, settle } = await render();
      const minute = el.querySelector('.cx__preset') as HTMLButtonElement;
      minute.click();
      await settle();
      expect(flat(el.querySelector('.cx > [role="alert"] p'))).toBe(
        'Não deu para passar o tempo: o servidor não respondeu. Tente de novo.',
      );
      minute.click();
      await settle();
      expect(advanceTime.mock.calls[1][2]).toBe(advanceTime.mock.calls[0][2]);
      expect(flat(el.querySelector('.cx__done'))).toBe('Passou 1 minuto. 2 efeitos acabaram.');
    });
  });

  it('opens the exhaustion dialog with the party and the levels it has', async () => {
    const { el, settle } = await render();
    const open = vi.spyOn(TestBed.inject(MatDialog), 'open');
    (el.querySelector('[data-testid="open-exhaustion"]') as HTMLButtonElement).click();
    await settle();
    const config = open.mock.calls[0][1] as {
      data: { options: { key: string; label: string; level: number }[]; state: unknown };
    };
    expect(config.data.options.map((o) => [o.key, o.label, o.level])).toEqual([
      ['ch1', 'Toren', 2],
      ['ch2', 'Ragna', 0],
    ]);
    expect(config.data.state).toBeNull();
    TestBed.inject(MatDialog).closeAll();
  });

  it('reads the catalog and opens "Adicionar efeito" with the party and the effects', async () => {
    const { el, settle } = await render();
    const open = vi.spyOn(TestBed.inject(MatDialog), 'open');
    (el.querySelector('[data-testid="open-give-effect"]') as HTMLButtonElement).click();
    await settle();
    expect(catalog).toHaveBeenCalledWith('camp');
    const config = open.mock.calls[0][1] as {
      data: {
        campaignId: string;
        characters: { id: string; name: string }[];
        catalog: { key: string }[];
      };
    };
    expect(config.data.campaignId).toBe('camp');
    expect(config.data.characters).toEqual([
      { id: 'ch1', name: 'Toren' },
      { id: 'ch2', name: 'Ragna' },
    ]);
    expect(config.data.catalog.map((c) => c.key)).toEqual(['spell:bless']);
    TestBed.inject(MatDialog).closeAll();
  });

  it('says so, in Portuguese, when the catalog could not be read, and opens nothing', async () => {
    catalog.mockRejectedValueOnce(new ConnectError('x', Code.Unavailable));
    const { el, settle } = await render();
    const open = vi.spyOn(TestBed.inject(MatDialog), 'open');
    (el.querySelector('[data-testid="open-give-effect"]') as HTMLButtonElement).click();
    await settle();
    expect(flat(el.querySelector('[role="alert"] p'))).toBe(
      'Não deu para ler o catálogo de efeitos: o servidor não respondeu. Tente de novo.',
    );
    expect(open).not.toHaveBeenCalled();
  });
});

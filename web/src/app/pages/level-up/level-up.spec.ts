import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { DicePreference, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CharacterBlockedReason,
  CharacterBlockedSchema,
  CharacterSchema,
  FullSheetSchema,
  LevelUpDiceRule,
  LevelUpRefusalReason,
  LevelUpRefusalSchema,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { LevelUpClient } from '../../core/levelup/levelup-client';
import { SKILLS, SPELLS, WIZARD_KEYS, fighterOptions, pensantus, wizardOptions } from '../../core/levelup/levelup-testing';
import { LevelUpPage } from './level-up';

const settle = () => new Promise((r) => setTimeout(r, 220));

function character(over: object = {}, derived = pensantus()) {
  return create(CharacterSchema, {
    id: 'ch-1',
    name: 'Pensantus',
    revision: 5,
    canLevelUp: true,
    derived,
    sheet: {
      content: {
        case: 'full',
        value: create(FullSheetSchema, {
          cantripKeys: [...WIZARD_KEYS.cantrips],
          knownSpellKeys: [...WIZARD_KEYS.known],
          preparedSpellKeys: [...WIZARD_KEYS.prepared],
          skillProficiencyKeys: [...WIZARD_KEYS.skills],
        }),
      },
    },
    ...over,
  });
}

describe('LevelUpPage', () => {
  const client = {
    character: vi.fn(),
    options: vi.fn(),
    catalog: vi.fn(),
    dicePreference: vi.fn(),
    preview: vi.fn(),
    rollHitPoints: vi.fn(),
    levelUp: vi.fn(),
    spellDetails: vi.fn(),
    xpMode: vi.fn(),
  };
  let navigate: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    // jsdom has no layout: scrolling does nothing.
    Element.prototype.scrollIntoView = vi.fn();
    window.scrollTo = vi.fn();
  });

  async function setup(options: LevelUpOptions = wizardOptions({ preparedMaxAfter: 3 }), char = character(), optionsError?: Error) {
    client.character.mockReset().mockResolvedValue(char);
    client.options.mockReset();
    if (optionsError) {
      client.options.mockRejectedValue(optionsError);
    } else {
      client.options.mockResolvedValue(options);
    }
    client.catalog.mockReset().mockResolvedValue({ spells: SPELLS, skills: SKILLS });
    client.dicePreference.mockReset().mockResolvedValue(DicePreference.APP);
    // The new maximum of prepared spells: 4, two more than the two prepared today.
    const after = pensantus(true);
    after.spellcasting[0].preparedMax = 4;
    client.preview.mockReset().mockResolvedValue({ after, refusal: undefined });
    client.xpMode.mockReset().mockResolvedValue(XpMode.MILESTONES);
    client.rollHitPoints.mockReset().mockResolvedValue({ die: 6, value: 5, alreadyRolled: false });
    client.levelUp.mockReset().mockResolvedValue(character({ canLevelUp: false }, pensantus(true)));
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: LevelUpClient, useValue: client },
      ],
    });
    // The route's params, read the way the page reads them.
    const { ActivatedRoute } = await import('@angular/router');
    TestBed.overrideProvider(ActivatedRoute, { useValue: { paramMap: (await import('rxjs')).of({ get: (k: string) => (k === 'id' ? 'camp-1' : 'ch-1') }) } });
    navigate = vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    const fixture = TestBed.createComponent(LevelUpPage);
    await load(fixture);
    return fixture;
  }

  async function load(fixture: ComponentFixture<LevelUpPage>) {
    fixture.detectChanges();
    await fixture.whenStable();
    await settle();
    fixture.detectChanges();
  }

  const el = (f: ComponentFixture<LevelUpPage>) => f.nativeElement as HTMLElement;
  const text = (f: ComponentFixture<LevelUpPage>) => el(f).textContent?.replace(/\s+/g, ' ') ?? '';
  const button = (f: ComponentFixture<LevelUpPage>, name: string | RegExp) =>
    Array.from(el(f).querySelectorAll<HTMLButtonElement>('button')).find((b) => (typeof name === 'string' ? b.textContent?.trim() === name : name.test(b.textContent ?? '')))!;
  async function click(f: ComponentFixture<LevelUpPage>, target: Element | null | undefined) {
    (target as HTMLElement).click();
    f.detectChanges();
    await f.whenStable();
    await settle();
    f.detectChanges();
  }
  const pickRow = (f: ComponentFixture<LevelUpPage>, name: string) =>
    Array.from(el(f).querySelectorAll<HTMLElement>('.row__main, .row')).find((r) => r.textContent?.includes(name))!;

  it('lists the four steps of Pensantus, and starts at Atributos', async () => {
    const f = await setup();
    expect(text(f)).toContain('Subir para o nível 4');
    expect(text(f)).toContain('Pensantus · Mago 3 → Mago 4');
    expect(text(f)).toContain('Passo 1 de 4 · Atributos');
    expect(Array.from(el(f).querySelectorAll('.step__label')).map((s) => s.textContent)).toEqual(['Atributos', 'Vida', 'Magias', 'Resumo']);
  });

  it('has only Vida and Resumo when the level has nothing else to choose (Toren)', async () => {
    const f = await setup(fighterOptions());
    expect(text(f)).toContain('Passo 1 de 2 · Vida');
    expect(text(f)).toContain('Neste nível não há mais nada para escolher');
    expect(text(f)).toContain('Ataque Extra');
  });

  it('keeps "Próximo" asking for the missing choice, and a tap puts the focus on it', async () => {
    const f = await setup();
    const next = button(f, 'Próximo');
    expect(next.getAttribute('aria-disabled')).toBe('true');
    expect(text(f)).toContain('Falta escolher 1 atributo.');
    await click(f, next);
    expect(text(f)).toContain('Passo 1 de 4');
    expect(el(f).querySelector('#pick-abilities')?.contains(document.activeElement)).toBe(true);
  });

  it('goes on when the step is complete, and Voltar goes back', async () => {
    const f = await setup();
    await click(f, el(f).querySelector('.row--on, .row'));
    // The first ability row is Força.
    await click(f, button(f, 'Próximo'));
    expect(text(f)).toContain('Passo 2 de 4 · Vida');
    await click(f, button(f, 'Voltar'));
    expect(text(f)).toContain('Passo 1 de 4 · Atributos');
  });

  describe('the discard question', () => {
    it('leaves at once when nothing was chosen', async () => {
      const f = await setup();
      await click(f, button(f, 'Cancelar'));
      expect(navigate).toHaveBeenCalledWith(['/campanhas', 'camp-1', 'personagens', 'ch-1']);
    });

    it('asks in place once something was chosen, with "Continuar escolhendo" first and focused', async () => {
      const f = await setup();
      await click(f, el(f).querySelector('.row__input'));
      await click(f, button(f, 'Cancelar'));
      expect(text(f)).toContain('Descartar as escolhas?');
      expect(navigate).not.toHaveBeenCalled();
      const buttons = Array.from(el(f).querySelectorAll('.ask button')).map((b) => b.textContent?.trim());
      expect(buttons).toEqual(['Continuar escolhendo', 'Descartar e sair']);
      expect(document.activeElement?.textContent?.trim()).toBe('Continuar escolhendo');
      await click(f, button(f, 'Continuar escolhendo'));
      expect(text(f)).not.toContain('Descartar as escolhas?');
      await click(f, button(f, 'Cancelar'));
      await click(f, button(f, 'Descartar e sair'));
      expect(navigate).toHaveBeenCalledWith(['/campanhas', 'camp-1', 'personagens', 'ch-1']);
    });

    it('asks too from "Voltar para a ficha" at the top, instead of following the link', async () => {
      const f = await setup();
      await click(f, el(f).querySelector('.row__input'));
      const back = el(f).querySelector('a.back') as HTMLAnchorElement;
      expect(back.getAttribute('href')).toBe('/campanhas/camp-1/personagens/ch-1');
      const event = new MouseEvent('click', { cancelable: true, bubbles: true });
      back.dispatchEvent(event);
      f.detectChanges();
      expect(event.defaultPrevented).toBe(true);
      expect(text(f)).toContain('Descartar as escolhas?');
      expect(navigate).not.toHaveBeenCalled();
    });
  });

  describe('the hit points', () => {
    async function atVida() {
      const f = await setup();
      await click(f, el(f).querySelector('.row__input'));
      await click(f, button(f, 'Próximo'));
      return f;
    }

    it('shows the average, preselected, with what the server derives', async () => {
      const f = await atVida();
      expect(text(f)).toContain('Média: 4');
      expect(text(f)).toContain('4 + Constituição +3 · de 23 para 30');
      expect(text(f)).toContain('1d6 + Constituição +3 · o resultado fica no registro');
      expect(text(f)).toContain('Feito no passo 1');
    });

    it('rolls the die on the server and keeps the result', async () => {
      const f = await atVida();
      await click(f, Array.from(el(f).querySelectorAll('.dice-choice__card')).find((c) => c.textContent?.includes('Rolar 1d6')));
      expect(text(f)).toContain('Falta rolar o dado de vida.');
      await click(f, button(f, /Rolar no app/));
      expect(client.rollHitPoints).toHaveBeenCalledWith('camp-1', 'ch-1', 'class:wizard', expect.any(String));
      expect(text(f)).toContain('Rolado no app: 5 no d6');
      expect(button(f, 'Próximo').getAttribute('aria-disabled')).toBeNull();
      // The preview is sent with the roll.
      expect(client.preview.mock.calls.some((c) => c[2].hitPoints.value === 5)).toBe(true);
    });

    it('takes a kept roll back on the spot', async () => {
      const f = await setup(wizardOptions({ preparedMaxAfter: 3, keptHitPointRoll: 3 }));
      await click(f, el(f).querySelector('.row__input'));
      await click(f, button(f, 'Próximo'));
      await click(f, Array.from(el(f).querySelectorAll('.dice-choice__card')).find((c) => c.textContent?.includes('Rolar 1d6')));
      expect(text(f)).toContain('Rolado no app: 3 no d6');
      expect(client.rollHitPoints).not.toHaveBeenCalled();
    });

    it('accepts a physical die, 1 to the die, when the campaign lets the player choose', async () => {
      const f = await atVida();
      await click(f, Array.from(el(f).querySelectorAll('.dice-choice__card')).find((c) => c.textContent?.includes('Rolar 1d6')));
      await click(f, button(f, 'Digitar o resultado'));
      const field = el(f).querySelector('input.type__field') as HTMLInputElement;
      field.value = '9';
      field.dispatchEvent(new Event('input'));
      f.detectChanges();
      expect(text(f)).toContain('Digite um número de 1 a 6');
      field.value = '4';
      field.dispatchEvent(new Event('input'));
      f.detectChanges();
      await click(f, button(f, 'Confirmar 4'));
      expect(text(f)).toContain('Dado físico: 4 no d6');
      expect(client.rollHitPoints).not.toHaveBeenCalled();
    });

    it('follows the dice rule: no typing when everybody rolls in the app, no app roll when everybody rolls physical dice', async () => {
      const inApp = await setup(wizardOptions({ preparedMaxAfter: 3, diceRule: LevelUpDiceRule.FORCED_IN_APP }));
      await click(inApp, el(inApp).querySelector('.row__input'));
      await click(inApp, button(inApp, 'Próximo'));
      await click(inApp, Array.from(el(inApp).querySelectorAll('.dice-choice__card')).find((c) => c.textContent?.includes('Rolar 1d6')));
      expect(button(inApp, /Rolar no app/)).toBeDefined();
      expect(button(inApp, 'Digitar o resultado')).toBeUndefined();
      TestBed.resetTestingModule();

      const physical = await setup(wizardOptions({ preparedMaxAfter: 3, diceRule: LevelUpDiceRule.FORCED_PHYSICAL }));
      await click(physical, el(physical).querySelector('.row__input'));
      await click(physical, button(physical, 'Próximo'));
      await click(physical, Array.from(el(physical).querySelectorAll('.dice-choice__card')).find((c) => c.textContent?.includes('Rolar 1d6')));
      expect(button(physical, /Rolar no app/)).toBeUndefined();
      expect(el(physical).querySelector('input.type__field')).not.toBeNull();
    });

    it('shows what an increase in Constituição does to the hit points', async () => {
      const f = await setup();
      const after = pensantus(true);
      // Constitution 16 → 18 with the average: 23 → 34.
      client.preview.mockResolvedValue({ after: { ...after, hitPointsMax: 34, abilities: after.abilities.map((a) => (a.namePt === 'Constituição' ? { ...a, score: 18, modifier: 4 } : a)) } });
      const con = pickRow(f, 'Constituição');
      await click(f, con.querySelector('input'));
      await click(f, button(f, 'Próximo'));
      expect(text(f)).toContain('Com Constituição 18');
      expect(text(f)).toContain('16 → 18 (+3 → +4)');
      expect(text(f)).toContain('23 → 34');
    });
  });

  describe('Magias and Resumo', () => {
    async function throughSpells() {
      const f = await setup();
      await click(f, pickRow(f, 'Inteligência').querySelector('input'));
      await click(f, button(f, 'Próximo'));
      await click(f, button(f, 'Próximo'));
      return f;
    }

    it('counts the picks, blocks "Próximo" with the reason, and unblocks it when complete', async () => {
      const f = await throughSpells();
      expect(text(f)).toContain('Passo 3 de 4 · Magias');
      expect(text(f)).toContain('0 de 1');
      expect(text(f)).toContain('0 de 2');
      expect(text(f)).toContain('Falta escolher 1 truque.');
      await click(f, pickRow(f, 'Prestidigitação').querySelector('input'));
      expect(text(f)).toContain('Faltam escolher 2 magias para o livro.');
      await click(f, pickRow(f, 'Passo Nebuloso').querySelector('input'));
      await click(f, pickRow(f, 'Imagem Espelhada').querySelector('input'));
      expect(text(f)).toContain('Faltam preparar 2 magias.');
      await click(f, el(f).querySelector('#pick-prepared .row__input'));
      expect(text(f)).toContain('Falta preparar 1 magia.');
      await click(f, el(f).querySelectorAll('#pick-prepared .row__input')[1]);
      expect(button(f, 'Próximo').getAttribute('aria-disabled')).toBeNull();
    });

    it('confirms with the choices only, never the sheet, and tells the sheet what to say', async () => {
      const f = await throughSpells();
      await click(f, pickRow(f, 'Prestidigitação').querySelector('input'));
      await click(f, pickRow(f, 'Passo Nebuloso').querySelector('input'));
      await click(f, pickRow(f, 'Imagem Espelhada').querySelector('input'));
      await click(f, el(f).querySelector('#pick-prepared .row__input'));
      await click(f, el(f).querySelectorAll('#pick-prepared .row__input')[1]);
      await click(f, button(f, 'Próximo'));
      expect(text(f)).toContain('Passo 4 de 4 · Resumo');
      expect(text(f)).toMatch(/18 para → ?20/);
      expect(text(f)).toContain('O resto da ficha não muda e continua travado.');
      await click(f, button(f, 'Confirmar o nível 4'));
      const [campaignId, characterId, revision, choices] = client.levelUp.mock.calls[0];
      expect([campaignId, characterId, revision]).toEqual(['camp-1', 'ch-1', 5]);
      expect(choices).toMatchObject({ classKey: 'class:wizard', abilityIncrease: { intelligence: 2 }, cantripKeys: ['spell:prestidigitation'] });
      expect(navigate).toHaveBeenCalledWith(['/campanhas', 'camp-1', 'personagens', 'ch-1'], {
        replaceUrl: true,
        state: { levelUp: { name: 'Pensantus', level: 4 } },
      });
    });

    it('mentions what the master adds by the editor, quietly', async () => {
      const f = await setup(fighterOptions({ masterAdds: [{ key: 'feature:favored-enemy', namePt: 'Inimigo Favorito' }] }));
      await click(f, button(f, 'Próximo'));
      expect(text(f)).toContain('O mestre acrescenta pelo editor: Inimigo Favorito.');
    });

    async function atResumoOfToren() {
      const f = await setup(fighterOptions());
      await click(f, button(f, 'Próximo'));
      return f;
    }

    it('sends the level-up once on a double click, while the first answer is on its way', async () => {
      const f = await atResumoOfToren();
      let answer: (c: unknown) => void = () => undefined;
      client.levelUp.mockReset().mockReturnValue(new Promise((r) => (answer = r)));
      const confirm = button(f, 'Confirmar o nível 5');
      confirm.click();
      confirm.click();
      confirm.click();
      expect(client.levelUp).toHaveBeenCalledTimes(1);
      answer(character({ canLevelUp: false }, pensantus(true)));
      await f.whenStable();
    });

    it('does not send an incomplete level-up even when the button still gets the click', async () => {
      const f = await setup();
      const page = f.componentInstance as unknown as { confirm(): Promise<void> };
      await page.confirm();
      expect(client.levelUp).not.toHaveBeenCalled();
    });

    it('reads the sheet and what the level gives again after a stale revision, and keeps the picks that are still valid', async () => {
      const f = await atResumoOfToren();
      client.levelUp.mockRejectedValueOnce(new ConnectError('x', Code.Aborted));
      await click(f, button(f, 'Confirmar o nível 5'));
      // The master changed the sheet: the class's average is another now, and the revision is 6.
      client.character.mockResolvedValue(character({ revision: 6 }));
      client.options.mockResolvedValue(fighterOptions({ hitPointAverage: 7 }));
      await click(f, button(f, 'Ler a ficha de novo'));
      expect(client.options).toHaveBeenCalledTimes(2);
      expect(el(f).querySelector('.js-failure')).toBeNull();
      await click(f, button(f, 'Voltar'));
      expect(text(f)).toContain('Média: 7');
      await click(f, button(f, 'Próximo'));
      await click(f, button(f, 'Confirmar o nível 5'));
      expect(client.levelUp.mock.calls.at(-1)?.[2]).toBe(6);
    });

    it('shows a stale revision in place, and "Ler a ficha de novo" reads it again for the next try', async () => {
      const f = await atResumoOfToren();
      client.levelUp.mockRejectedValueOnce(new ConnectError('x', Code.Aborted));
      await click(f, button(f, 'Confirmar o nível 5'));
      expect(el(f).querySelector('.js-failure')?.textContent).toContain('A ficha mudou enquanto você escolhia');
      client.character.mockResolvedValue(character({ revision: 6 }));
      await click(f, button(f, 'Ler a ficha de novo'));
      expect(el(f).querySelector('.js-failure')).toBeNull();
      await click(f, button(f, 'Confirmar o nível 5'));
      expect(client.levelUp.mock.calls.at(-1)?.[2]).toBe(6);
    });

    it('shows a refusal with its reason, and takes the player to the step that owns it', async () => {
      const f = await setup();
      await click(f, pickRow(f, 'Inteligência').querySelector('input'));
      await click(f, button(f, 'Próximo'));
      await click(f, button(f, 'Próximo'));
      await click(f, pickRow(f, 'Prestidigitação').querySelector('input'));
      await click(f, pickRow(f, 'Passo Nebuloso').querySelector('input'));
      await click(f, pickRow(f, 'Imagem Espelhada').querySelector('input'));
      await click(f, el(f).querySelector('#pick-prepared .row__input'));
      await click(f, el(f).querySelectorAll('#pick-prepared .row__input')[1]);
      await click(f, button(f, 'Próximo'));
      client.levelUp.mockRejectedValueOnce(
        new ConnectError('x', Code.FailedPrecondition, undefined, [
          { desc: LevelUpRefusalSchema, value: create(LevelUpRefusalSchema, { reason: LevelUpRefusalReason.CANTRIPS, field: 'full.cantrip_keys' }) },
        ]),
      );
      await click(f, button(f, 'Confirmar o nível 4'));
      expect(el(f).querySelector('.js-failure')?.textContent).toContain('Escolha todos os truques novos do nível');
      await click(f, button(f, 'Ir para Magias'));
      expect(text(f)).toContain('Passo 3 de 4 · Magias');
      expect(el(f).querySelector('.js-failure')).toBeNull();
    });
  });

  describe('when there is nothing to level up', () => {
    it('says why a character that cannot level up now cannot, with no steps (as for a locked sheet)', async () => {
      const blocked = new ConnectError('x', Code.FailedPrecondition, undefined, [
        { desc: CharacterBlockedSchema, value: create(CharacterBlockedSchema, { reason: CharacterBlockedReason.CANNOT_LEVEL_UP }) },
      ]);
      const f = await setup(wizardOptions(), character({ canLevelUp: false }), blocked);
      expect(text(f)).toContain('Ainda não dá para subir de nível');
      expect(text(f)).toContain('Falta o mestre marcar um marco para este personagem');
      expect(el(f).querySelector('a.blocked__back')?.textContent).toContain('Voltar para a ficha');
      expect(el(f).querySelector('app-steps-bar')).toBeNull();
    });

    it('says a character that is not there is not there', async () => {
      const f = await setup(wizardOptions(), character(), new ConnectError('x', Code.NotFound));
      expect(text(f)).toContain('Esse personagem não existe, ou você não pode vê-lo.');
    });

    it('tells the master that the player levels up, without asking for options', async () => {
      const f = await setup(wizardOptions(), character({ canAccessMasterNotes: true }));
      expect(text(f)).toContain('Ainda não dá para subir de nível');
      expect(text(f)).toContain('Quem sobe o nível é o jogador');
      expect(client.options).not.toHaveBeenCalled();
    });
  });
});

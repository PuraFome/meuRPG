import { create } from '@bufbuild/protobuf';
import { TestBed } from '@angular/core/testing';

import {
  CombatantKind,
  GetMoveOptionsResponseSchema,
  MoveRefusal,
  ReachableSquareSchema,
  RefusedSquareSchema,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { JumpLimitsSchema } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import { combatant, encounter } from '../../../../core/combat/combat-testing';
import type { MapLayers } from '../../../../core/maps/layers';
import { type JumpRequest, MovePage } from './move-page';

const plain = (t: string | null | undefined) => (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

// Toren at (8, 7) with 9,0 m: Goblin 2 stands at (7, 7), the wall is at (8, 5).
const toren = combatant({
  id: 'toren',
  label: 'Toren',
  kind: CombatantKind.PLAYER,
  mine: true,
  col: 8,
  row: 7,
  speedFt: 30,
  speedDft: 300,
  movementLeftFt: 30,
  movementLeftDft: 300,
});
const goblin = combatant({ id: 'g2', label: 'Goblin 2', col: 7, row: 7 });

const options = create(GetMoveOptionsResponseSchema, {
  movementLeftDft: 300,
  reachable: [
    create(ReachableSquareSchema, { col: 9, row: 8, costDft: 71 }),
    create(ReachableSquareSchema, { col: 10, row: 7, costDft: 100, provokesReactorIds: ['g2'] }),
    create(ReachableSquareSchema, { col: 8, row: 9, costDft: 100, knownTrapName: 'Fosso escondido', knownTrapPointId: 'p' }),
  ],
  refused: [create(RefusedSquareSchema, { col: 8, row: 5, reason: MoveRefusal.WALL })],
});

const layers: MapLayers = { columns: 20, rows: 14, walls: [{ col: 8, row: 5 }], terrain: [{ col: 9, row: 8 }], half: [], threeQuarters: [] };

function setup(over: { options?: typeof options | null; jumps?: ReturnType<typeof create<typeof JumpLimitsSchema>>; canDisengage?: boolean } = {}) {
  const fixture = TestBed.createComponent(MovePage);
  const ref = fixture.componentRef;
  ref.setInput('encounter', encounter({ combatants: [toren, goblin], currentCombatantId: 'toren', turnGroupIds: ['toren'] }));
  ref.setInput('image', { url: '/images/x', width: 2000, height: 1400 });
  ref.setInput('mapName', 'A caverna do Vale Seco');
  ref.setInput('sessionNumber', 6);
  ref.setInput('options', over.options === undefined ? options : over.options);
  ref.setInput('layers', layers);
  ref.setInput('jumps', over.jumps);
  ref.setInput('canDisengage', over.canDisengage ?? true);
  const confirmed: unknown[] = [];
  const jumped: JumpRequest[] = [];
  let disengaged = 0;
  fixture.componentInstance.confirm.subscribe((s) => confirmed.push(s));
  fixture.componentInstance.jump.subscribe((j) => jumped.push(j));
  fixture.componentInstance.disengage.subscribe(() => disengaged++);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const choose = (col: number, row: number) => {
    const surface = el.querySelector<HTMLElement>('.cm__surface')!;
    // The map is one focus stop: a click picks the square under it.
    surface.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 140, right: 200, bottom: 140, x: 0, y: 0, toJSON: () => '' });
    surface.dispatchEvent(new MouseEvent('click', { clientX: col * 10 + 5, clientY: row * 10 + 5, bubbles: true }));
    fixture.detectChanges();
  };
  const press = (name: string) => {
    Array.from(el.querySelectorAll('button')).find((b) => plain(b.textContent).includes(name))!.click();
    fixture.detectChanges();
  };
  return { fixture, el, choose, press, confirmed, jumped, disengaged: () => disengaged };
}

describe('MovePage', () => {
  it('draws the server\'s reach: one tinted square for each reachable one, the ring and the layers', () => {
    const { el } = setup();
    expect(el.querySelectorAll('.cm__cell').length).toBe(3);
    expect(el.querySelector('.cm__ring ellipse')).not.toBeNull();
    expect(el.querySelectorAll('app-map-layers .sq--wall').length).toBe(1);
    expect(el.querySelectorAll('app-map-layers .sq--terrain').length).toBe(1);
    // The legend names every mark that is drawn, layers first (MAP-LANGUAGE.md).
    const legend = Array.from(el.querySelectorAll('.mr-legend li'), (li) => plain(li.textContent));
    expect(legend).toEqual(['Parede', 'Terreno difícil', 'Você alcança', 'Alcance de 9,0 m', 'Quadrado escolhido']);
    expect(plain(el.textContent)).toContain('Dentro do círculo, o que fica sem cor não dá para alcançar: parede, inimigo ou custo a mais.');
  });

  it('says what is left before anything is chosen', () => {
    const { el } = setup();
    expect(plain(el.querySelector('h1')?.textContent)).toBe('Mover Toren');
    expect(plain(el.querySelector('.move__lead')?.textContent)).toBe('Restam 9,0 m de 9,0 m (6 quadrados de 1,5 m). Toque num quadrado destacado.');
    expect(plain(el.textContent)).toContain('Toque num quadrado destacado para escolher onde parar.');
    expect(el.querySelector<HTMLButtonElement>('.move__go')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('says the cost the server sent and what is left, with the cost beside the square', () => {
    const { el, choose, press, confirmed } = setup();
    choose(9, 8);
    expect(plain(el.querySelector('.status__title')?.textContent)).toBe('Mover 2,1 m');
    expect(plain(el.querySelector('.status__text')?.textContent)).toBe('Depois restam 6,9 m.');
    expect(plain(el.querySelector('.cm__cost')?.textContent)).toBe('2,1 m');
    press('Mover para cá');
    expect(confirmed).toEqual([{ col: 9, row: 8 }]);
  });

  it('refuses a square the server refused, by its reason, and does not confirm', () => {
    const { el, choose, press, confirmed } = setup();
    choose(8, 5);
    const alert = el.querySelector('[role="alert"]');
    expect(plain(alert?.textContent)).toContain('Sem caminho reto');
    expect(plain(alert?.textContent)).toContain('Para contornar, mova em partes.');
    expect(plain(el.querySelector('.cm__cost')?.textContent)).toBe('Sem caminho reto');
    press('Mover para cá');
    expect(confirmed).toEqual([]);
  });

  it('says "Longe demais" beyond the circle, without inventing a number the server did not send', () => {
    const { el, choose } = setup();
    choose(19, 13);
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toContain('Longe demais');
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).not.toContain('faltam');
  });

  it('warns that leaving a reach may provoke, and offers Desengajar', () => {
    const { el, choose, press, disengaged } = setup();
    choose(10, 7);
    expect(plain(el.textContent)).toContain('Sair do alcance do Goblin 2 pode provocar um ataque de oportunidade.');
    expect(plain(el.textContent)).toContain('Com Desengajar, nenhum movimento deste turno provoca isso.');
    press('Desengajar (gasta a ação)');
    expect(disengaged()).toBe(1);
  });

  it('does not offer Desengajar when the action is gone', () => {
    const { el, choose } = setup({ canDisengage: false });
    choose(10, 7);
    expect(plain(el.textContent)).toContain('Sair do alcance do Goblin 2');
    expect(plain(el.textContent)).not.toContain('Desengajar');
  });

  it('asks before a move into a trap the character knows, with the focus on the safe answer', () => {
    const { fixture, el, choose, press, confirmed } = setup();
    choose(8, 9);
    expect(plain(el.textContent)).toContain('Esse quadrado fica dentro do Fosso escondido.');
    press('Mover para cá');
    expect(plain(el.querySelector('.move__ask')?.textContent)).toBe('Isso entra no Fosso escondido. Mover assim mesmo?');
    expect(confirmed).toEqual([]);
    fixture.detectChanges();
    press('Mover assim mesmo');
    expect(confirmed).toEqual([{ col: 8, row: 9 }]);
  });

  it('moves the choice with the four arrows, from the token', () => {
    const { el, fixture } = setup();
    const arrows = Array.from(el.querySelectorAll<HTMLButtonElement>('.adj__btn'));
    expect(arrows.map((b) => b.getAttribute('aria-label'))).toEqual([
      'Um quadrado para a esquerda',
      'Um quadrado para cima',
      'Um quadrado para baixo',
      'Um quadrado para a direita',
    ]);
    arrows[3].click();
    arrows[2].click();
    fixture.detectChanges();
    // (8, 7) -> (9, 7) -> (9, 8): the tinted one.
    expect(plain(el.querySelector('.status__title')?.textContent)).toBe('Mover 2,1 m');
  });

  it('still lets the server judge when the options could not be read', () => {
    const { fixture, el } = setup({ options: null });
    fixture.componentRef.setInput('optionsFailed', true);
    fixture.detectChanges();
    expect(plain(el.textContent)).toContain('Não deu para ler o alcance.');
  });

  describe('Saltar', () => {
    // Toren, Força 16, with a running start: 4,8 m, high 1,8 m; standing the half.
    const jumps = create(JumpLimitsSchema, {
      longRunningDft: 160,
      longStandingDft: 80,
      highRunningDft: 60,
      highStandingDft: 30,
      runningStart: true,
    });

    it('has no Saltar when the sheet has no jump limits', () => {
      const { el } = setup();
      expect(el.querySelector('app-segmented')).toBeNull();
    });

    it('shows the limits, the running start and the circle of the jump', () => {
      const { fixture, el } = setup({ jumps });
      const radios = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
      expect(radios.length).toBe(2);
      radios[1].click();
      radios[1].dispatchEvent(new Event('change'));
      fixture.detectChanges();
      expect(plain(el.querySelector('h1')?.textContent)).toBe('Saltar Toren');
      const text = plain(el.textContent);
      expect(text).toContain('Distância4,8 m com corrida · 2,4 m parado');
      expect(text).toContain('Altura1,8 m com corrida · 0,9 m parado');
      expect(text).toContain('Você andou 3,0 m a pé antes de saltar.');
      expect(el.querySelectorAll('.cm__cell').length).toBe(0);
      expect(plain(el.querySelector('.mr-legend')?.textContent)).toContain('Alcance de 4,8 m');
    });

    it('jumps to the chosen square, costing the length of the line', () => {
      const { fixture, el, choose, press, jumped } = setup({ jumps });
      const radios = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
      radios[1].click();
      radios[1].dispatchEvent(new Event('change'));
      fixture.detectChanges();
      choose(11, 7);
      expect(plain(el.querySelector('.status__title')?.textContent)).toBe('Saltar 4,5 m');
      expect(plain(el.querySelector('.status__text')?.textContent)).toContain('Depois restam 4,5 m.');
      press('Saltar para cá');
      expect(jumped).toEqual([{ kind: 'long', square: { col: 11, row: 7 } }]);
    });

    it('steps the high jump by 0,3 m up to the limit and sends the height', () => {
      const { fixture, el, press, jumped } = setup({ jumps });
      const radios = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
      radios[1].click();
      radios[1].dispatchEvent(new Event('change'));
      fixture.detectChanges();
      // "Altura" is the second kind of the second group.
      const kinds = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]')).slice(2);
      kinds[1].click();
      kinds[1].dispatchEvent(new Event('change'));
      fixture.detectChanges();
      expect(plain(el.querySelector('.high__val')?.textContent)).toBe('1,8 m');
      expect(el.querySelector('.cm__ring')).toBeNull();
      el.querySelector<HTMLButtonElement>('[aria-label="Diminuir a altura em 0,3 m"]')!.click();
      fixture.detectChanges();
      expect(plain(el.querySelector('.high__val')?.textContent)).toBe('1,5 m');
      press('Saltar 1,5 m para cima');
      expect(jumped).toEqual([{ kind: 'high', heightDft: 50 }]);
    });
  });
});

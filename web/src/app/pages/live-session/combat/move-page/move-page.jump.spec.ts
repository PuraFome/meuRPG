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
import { MovePage } from './move-page';

const plain = (t: string | null | undefined) =>
  (t ?? '').replace(/ /g, ' ').replace(/\s+/g, ' ').trim();

/** The words of the jump card and of the page around it, square by square, for what the main spec does not choose. */
describe('MovePage: the jump card and the lines of the page', () => {
  const toren = (left: number) =>
    combatant({
      id: 'toren',
      label: 'Toren',
      kind: CombatantKind.PLAYER,
      mine: true,
      col: 8,
      row: 7,
      speedFt: 30,
      speedDft: 300,
      movementLeftFt: left / 10,
      movementLeftDft: left,
    });
  const jumps = create(JumpLimitsSchema, {
    longRunningDft: 160,
    longStandingDft: 80,
    highRunningDft: 60,
    highStandingDft: 30,
    runningStart: true,
  });
  const options = (left: number) =>
    create(GetMoveOptionsResponseSchema, {
      movementLeftDft: left,
      reachable: [
        create(ReachableSquareSchema, {
          col: 10,
          row: 7,
          costDft: 100,
          provokesReactorIds: ['g2'],
        }),
        create(ReachableSquareSchema, { col: 9, row: 8, costDft: 71 }),
      ],
      refused: [create(RefusedSquareSchema, { col: 8, row: 5, reason: MoveRefusal.WALL })],
    });

  function open(left: number, over: { jumps?: typeof jumps; canDisengage?: boolean } = {}) {
    TestBed.resetTestingModule();
    const fixture = TestBed.createComponent(MovePage);
    const ref = fixture.componentRef;
    ref.setInput(
      'encounter',
      encounter({
        combatants: [toren(left), combatant({ id: 'g2', label: 'Goblin 2', col: 7, row: 7 })],
        currentCombatantId: 'toren',
        turnGroupIds: ['toren'],
      }),
    );
    ref.setInput('image', { url: '/images/x', width: 2000, height: 1400 });
    ref.setInput('options', options(left));
    ref.setInput('jumps', over.jumps === undefined ? jumps : over.jumps);
    ref.setInput('canDisengage', over.canDisengage ?? true);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const choose = (col: number, row: number) => {
      const surface = el.querySelector<HTMLElement>('.cm__surface')!;
      surface.getBoundingClientRect = () =>
        ({
          left: 0,
          top: 0,
          width: 200,
          height: 140,
          right: 200,
          bottom: 140,
          x: 0,
          y: 0,
        }) as DOMRect;
      surface.dispatchEvent(
        new MouseEvent('click', { clientX: col * 10 + 5, clientY: row * 10 + 5, bubbles: true }),
      );
      fixture.detectChanges();
    };
    const radio = (i: number) => {
      const radios = Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'));
      radios[i].click();
      fixture.detectChanges();
    };
    const press = (name: string) => {
      Array.from(el.querySelectorAll('button'))
        .find((b) => plain(b.textContent).includes(name))!
        .click();
      fixture.detectChanges();
    };
    const text = () => plain(el.textContent);
    return { fixture, el, choose, radio, press, text };
  }
  const jumpMode = (left = 300) => {
    const t = open(left);
    t.radio(1);
    return t;
  };
  const highMode = (left = 300) => {
    const t = jumpMode(left);
    t.radio(3);
    return t;
  };

  it('says the long jump is to a square, and asks to tap where to land', () => {
    const { el } = jumpMode();
    expect(plain(el.querySelector('.move__lead')?.textContent)).toContain(
      'Toque no quadrado onde quer cair.',
    );
    expect(plain(el.querySelector('.move__title')?.textContent)).toBe('Saltar Toren');
  });

  it('refuses the square the jumper stands on', () => {
    const { fixture, el } = jumpMode();
    (fixture.componentInstance as unknown as { choose(s: object): void }).choose({
      col: 8,
      row: 7,
    });
    fixture.detectChanges();
    expect(plain(el.querySelector('.err__title')?.textContent)).toBe('Você já está aqui');
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toContain(
      'Toque no quadrado onde quer cair.',
    );
  });

  it('refuses a landing past the limit of the jump, and says the limit', () => {
    const { el, choose } = jumpMode();
    choose(15, 7);
    const alert = plain(el.querySelector('[role="alert"]')?.textContent);
    expect(alert).toContain('Longe demais');
    expect(alert).toContain('Seu salto vai até 4,8 m. Escolha um quadrado dentro do círculo.');
  });

  it('writes the card of a landing in reach, with its cost', () => {
    const { el, choose } = jumpMode();
    choose(9, 8);
    expect(plain(el.querySelector('.status__title')?.textContent)).toBe('Saltar 2,1 m');
  });

  it('warns that a landing out of an enemy reach may provoke, in the words of a jump', () => {
    const { text, choose } = jumpMode();
    choose(10, 7);
    expect(text()).toContain('Esse salto sai do alcance do Goblin 2.');
    expect(text()).toContain('Desengajar (gasta a ação)');
  });

  it('writes the high jump card: up, without changing square', () => {
    const { el } = highMode();
    expect(plain(el.querySelector('.status__title')?.textContent)).toBe('Saltar 1,8 m');
    expect(plain(el.querySelector('.status')?.textContent)).toContain(
      'Você sobe sem mudar de quadrado, para agarrar uma borda.',
    );
    expect(plain(el.querySelector('.move__lead')?.textContent)).not.toContain('Toque');
  });

  it('says there is no high jump when not even a step fits the movement left', () => {
    const { el } = highMode(5);
    expect(plain(el.querySelector('.err__title')?.textContent)).toBe('Sem salto em altura');
    expect(plain(el.querySelector('[role="alert"]')?.textContent)).toContain(
      'Com essa Força e esse movimento, não dá para subir nem um passo de 0,3 m.',
    );
  });

  it('names the jump on the button, by kind and height', () => {
    const long = jumpMode();
    expect(plain(long.el.querySelector('.move__go')?.textContent)).toContain('Saltar para cá');
    const high = highMode();
    expect(plain(high.el.querySelector('.move__go')?.textContent)).toContain(
      'Saltar 1,8 m para cima',
    );
    const walk = open(300);
    expect(plain(walk.el.querySelector('.move__go')?.textContent)).toContain('Mover para cá');
  });

  it('says how much is left to walk, with the squares, and nothing about squares when jumping', () => {
    const walk = open(300);
    expect(plain(walk.el.querySelector('.move__lead')?.textContent)).toContain(
      'Toque num quadrado destacado.',
    );
    expect(plain(walk.el.querySelector('.move__lead')?.textContent)).toContain('quadrados');
    const jump = jumpMode();
    expect(plain(jump.el.querySelector('.move__lead')?.textContent)).not.toContain('quadrados');
  });

  it('keeps the zoom between the first and the last step', () => {
    const { fixture, el } = open(300);
    const zoom = (label: string) => {
      el.querySelector<HTMLButtonElement>(`[aria-label="${label}"]`)?.click();
      fixture.detectChanges();
    };
    for (let i = 0; i < 6; i++) {
      zoom('Aproximar o mapa');
    }
    expect(el.querySelector('[aria-label="Aproximar o mapa"]')?.getAttribute('aria-disabled')).toBe(
      'true',
    );
    for (let i = 0; i < 6; i++) {
      zoom('Afastar o mapa');
    }
    expect(el.querySelector('[aria-label="Afastar o mapa"]')?.getAttribute('aria-disabled')).toBe(
      'true',
    );
  });

  it('keeps the chosen square in view as the map scrolls', () => {
    const { fixture, el, choose } = open(300);
    const scroller = el.querySelector<HTMLElement>('.move__scroll')!;
    Object.defineProperty(scroller, 'clientWidth', { value: 100, configurable: true });
    Object.defineProperty(scroller, 'clientHeight', { value: 100, configurable: true });
    scroller.scrollLeft = 0;
    scroller.scrollTop = 0;
    choose(19, 13);
    fixture.detectChanges();
    expect(scroller.scrollLeft).toBeGreaterThan(0);
    expect(scroller.scrollTop).toBeGreaterThan(0);
  });
});

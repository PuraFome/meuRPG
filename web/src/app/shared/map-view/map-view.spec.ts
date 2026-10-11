import { stubFullscreen } from '../../core/ui/fullscreen-testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';

import { mapPoint, mapToken } from '../../core/maps/maps-testing';
import { MapMove, MapView } from './map-view';

const IMAGE = { url: '/images/img-1', width: 2400, height: 1600 };

describe('MapView', () => {
  let fixture: ComponentFixture<MapView>;
  let el: HTMLElement;
  let moves: MapMove[];

  const taverna = mapPoint('p1', 'Taverna do Javali', { revealed: true, xBp: 5000, yBp: 5000 });
  const covil = mapPoint('p2', 'Covil dos goblins', {
    revealed: false,
    kind: 2,
    xBp: 2500,
    yBp: 7000,
  });
  const pensantus = mapToken('t1', 'Pensantus', { mine: true });
  const capitao = mapToken('t2', 'Capitão Goblin', { hidden: true });

  function setup(inputs: Record<string, unknown>): void {
    fixture = TestBed.createComponent(MapView);
    fixture.componentRef.setInput('image', IMAGE);
    fixture.componentRef.setInput('mapName', 'Mirathel e arredores');
    fixture.componentRef.setInput('points', [taverna, covil]);
    fixture.componentRef.setInput('tokens', [pensantus, capitao]);
    for (const [key, value] of Object.entries(inputs)) {
      fixture.componentRef.setInput(key, value);
    }
    moves = [];
    fixture.componentInstance.moved.subscribe((m) => moves.push(m));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }

  function item(key: string): HTMLButtonElement {
    return el.querySelector(`[data-item="${key}"]`) as HTMLButtonElement;
  }

  beforeEach(() => TestBed.configureTestingModule({ imports: [MapView] }));

  it('watches its size after the first render and stops watching when it goes away', async () => {
    const observers: { observed: Element[]; disconnected: boolean }[] = [];
    vi.stubGlobal(
      'ResizeObserver',
      class {
        private readonly state = { observed: [] as Element[], disconnected: false };
        constructor() {
          observers.push(this.state);
        }
        observe(target: Element): void {
          this.state.observed.push(target);
        }
        disconnect(): void {
          this.state.disconnected = true;
        }
      },
    );
    try {
      setup({});
      await fixture.whenStable();
      expect(observers).toHaveLength(1);
      expect(observers[0].observed).toHaveLength(1);
      fixture.destroy();
      expect(observers[0].disconnected).toBe(true);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it('reserves the image size before it loads', () => {
    setup({});
    expect((el.querySelector('.mv') as HTMLElement).style.aspectRatio).toBe('2400 / 1600');
    const img = el.querySelector('img') as HTMLImageElement;
    expect(img.getAttribute('width')).toBe('2400');
    expect(img.getAttribute('height')).toBe('1600');
  });

  it('draws only the revealed points and the visible tokens for a player', () => {
    setup({ isMaster: false });
    expect(item('point:p1')).toBeTruthy();
    expect(item('point:p2')).toBeNull();
    expect(el.textContent).not.toContain('Covil dos goblins');
    expect(el.textContent).not.toContain('Escondido');
  });

  it('draws hidden things for the master, with the word "Escondido"', () => {
    setup({ isMaster: true, mode: 'tokens' });
    expect(item('point:p2').getAttribute('aria-label')).toBe(
      'Covil dos goblins, Submapa, escondido',
    );
    expect(el.querySelectorAll('.lbl--hidden')).toHaveLength(1);
    expect(el.querySelector('.lbl--hidden')?.textContent).toContain('Escondido');
    expect(item('token:t2').getAttribute('aria-label')).toBe('Capitão Goblin, escondido');
    expect(item('token:t1').getAttribute('aria-label')).toBe('Pensantus (você), visível');
  });

  it('names the points of one square with one label, hidden only when all of them are', () => {
    const agulha = mapPoint('p3', 'Agulha envenenada', {
      kind: 4,
      xBp: 5050,
      yBp: 5050,
      revealed: false,
    });
    setup({ isMaster: true, mode: 'tokens', squares: 20, points: [taverna, agulha, covil] });
    const labels = Array.from(el.querySelectorAll('.lbl'));
    // The tavern and the needle trap stand on one square: one label with both names; the lair is on another.
    expect(labels).toHaveLength(2);
    expect(labels[0].textContent).toContain('Taverna do Javali · Agulha envenenada');
    expect(labels[0].classList).not.toContain('lbl--hidden');
    expect(labels[1].classList).toContain('lbl--hidden');
  });

  it('draws no labels when asked not to (painting), and fades the markers', () => {
    setup({ isMaster: true, labels: false, faded: true });
    expect(el.querySelectorAll('.lbl')).toHaveLength(0);
    expect(el.querySelector('.mv')?.classList).toContain('mv--faded');
  });

  it('shows a found treasure and a trap known to a character to a player, whatever "revealed" says; never a light', () => {
    const chest = mapPoint('c1', 'Baú de moedas', {
      kind: 5,
      revealed: false,
      treasureFoundAt: { seconds: 1n, nanos: 0 } as never,
      xBp: 1000,
      yBp: 1000,
    });
    const trap = mapPoint('t1', 'Fosso', {
      kind: 4,
      revealed: false,
      xBp: 9000,
      yBp: 1000,
      trap: { state: 2 } as never,
    });
    const torch = mapPoint('l1', 'Tocha', { kind: 6, revealed: true, xBp: 9000, yBp: 9000 });
    setup({ isMaster: false, points: [chest, trap, torch] });
    expect(item('point:c1')).toBeTruthy();
    expect(item('point:l1')).toBeNull();
    expect(Array.from(el.querySelectorAll('.lbl')).map((l) => l.textContent?.trim())).toEqual([
      'Baú de moedas',
      'Fosso',
    ]);
  });

  it('labels the markers with the name and the kind', () => {
    setup({});
    expect(item('point:p1').getAttribute('aria-label')).toBe('Taverna do Javali, Cena de RP');
  });

  it('opens a point on click, and a token only where tokens move', () => {
    setup({ isMaster: true, mode: 'view' });
    const selected: string[] = [];
    fixture.componentInstance.pointSelect.subscribe((id) => selected.push(id));
    item('point:p1').click();
    expect(selected).toEqual(['p1']);
    expect(item('token:t1')).toBeNull(); // not a button in the view mode
  });

  it('keeps the markers inert when points are not selectable', () => {
    setup({ isMaster: true, selectablePoints: false });
    expect(item('point:p1')).toBeNull();
    expect(el.querySelectorAll('app-map-marker').length).toBe(2);
  });

  it('is a still picture in preview mode: no buttons, no zoom controls', () => {
    setup({ mode: 'preview', controls: 'overlay' });
    expect(el.querySelectorAll('button')).toHaveLength(0);
    expect(el.querySelector('.mv__controls')).toBeNull();
    expect(el.querySelector('.mv')?.getAttribute('role')).toBe('img');
  });

  it('moves the selected item with the arrow keys and reports it on key release', () => {
    setup({ isMaster: true, mode: 'edit', selected: { kind: 'point', id: 'p1' } });
    const marker = item('point:p1');
    const press = (type: string, init: KeyboardEventInit) =>
      marker.dispatchEvent(new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init }));

    press('keydown', { key: 'ArrowRight' });
    press('keydown', { key: 'ArrowRight' });
    press('keydown', { key: 'ArrowDown', shiftKey: true });
    expect(moves).toEqual([]); // nothing is saved while the key is down
    fixture.detectChanges();
    expect((marker.closest('app-map-marker') as HTMLElement).style.left).toBe('51%');
    press('keyup', { key: 'ArrowDown' });
    expect(moves).toEqual([{ kind: 'point', id: 'p1', xBp: 5100, yBp: 5500 }]);
  });

  it('does not move a point in a mode that does not edit points', () => {
    setup({ isMaster: true, mode: 'tokens' });
    const marker = item('point:p1');
    expect(marker).toBeTruthy();
    marker.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    marker.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }));
    expect(moves).toEqual([]);
  });

  it('moves a token with the keyboard where tokens move', () => {
    setup({ isMaster: true, mode: 'tokens' });
    const token = item('token:t1');
    token.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));
    token.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowUp', bubbles: true }));
    expect(moves).toEqual([{ kind: 'token', id: 't1', xBp: 4000, yBp: 3950 }]);
  });

  it('starts a second move from the first one before the new tokens arrive from the parent', () => {
    setup({ isMaster: true, mode: 'tokens' });
    const token = item('token:t1');
    const step = () => {
      token.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
      token.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }));
    };
    // Two quick moves, no change detection in between: the second must not
    // repeat the first (it did, and the players' screens kept the old place).
    step();
    step();
    expect(moves.map((m) => m.xBp)).toEqual([4050, 4100]);
    fixture.detectChanges();
    expect((token.closest('app-map-token') as HTMLElement).style.left).toBe('41%');
  });

  it('forgets its last move once the parent sends new tokens (e.g. a refused move undone)', () => {
    setup({ isMaster: true, mode: 'tokens' });
    const token = item('token:t1');
    token.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    token.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }));
    fixture.componentRef.setInput('tokens', [{ ...pensantus }, capitao]);
    fixture.detectChanges();
    expect((token.closest('app-map-token') as HTMLElement).style.left).toBe('40%');
    token.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
    token.dispatchEvent(new KeyboardEvent('keyup', { key: 'ArrowRight', bubbles: true }));
    expect(moves.map((m) => m.xBp)).toEqual([4050, 4050]);
  });

  it('zooms with the buttons from 100 % to 400 % and fits again', () => {
    setup({ mode: 'view' });
    const view = fixture.componentInstance;
    const buttons = (label: string) =>
      el.querySelector(`button[aria-label="${label}"]`) as HTMLButtonElement;
    expect(buttons('Reduzir').disabled).toBe(true);
    buttons('Ampliar').click();
    expect(view.scale()).toBe(1.25);
    for (let i = 0; i < 20; i++) {
      view.zoomIn();
    }
    expect(view.scale()).toBe(4);
    fixture.detectChanges();
    expect(buttons('Ampliar').disabled).toBe(true);
    buttons('Ajustar à tela').click();
    expect(view.scale()).toBe(1);
  });

  it('raises an overlapped item while it is hovered or focused', () => {
    setup({ isMaster: true, mode: 'tokens' });
    item('token:t1').dispatchEvent(new Event('focusin', { bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('app-map-token.tk--raised')).toBeTruthy();
    item('token:t1').dispatchEvent(new Event('focusout', { bubbles: true }));
    fixture.detectChanges();
    expect(el.querySelector('app-map-token.tk--raised')).toBeNull();
  });

  it('gives two tokens with the same letter two letters', () => {
    setup({
      isMaster: true,
      mode: 'tokens',
      tokens: [mapToken('a', 'Brisa'), mapToken('b', 'Boris'), mapToken('c', 'Toren')],
    });
    const discs = Array.from(el.querySelectorAll('.tk__disc')).map((d) => d.textContent?.trim());
    expect(discs).toEqual(['Br', 'Bo', 'T']);
  });

  describe('full screen', () => {
    let fs: ReturnType<typeof stubFullscreen>;
    afterEach(() => fs.restore());

    it('offers "Tela cheia" only where the browser has the API', () => {
      fs = stubFullscreen(false);
      setup({});
      expect(el.querySelector('[aria-label="Tela cheia"]')).toBeNull();
      fs.restore();
      fs = stubFullscreen(true);
      setup({});
      expect(el.querySelector('[aria-label="Tela cheia"]')).toBeTruthy();
    });

    it("asks the host for full screen, follows the browser's state and leaves on a second click", () => {
      fs = stubFullscreen(true);
      setup({});
      fs.track(el);
      const button = () =>
        el.querySelector<HTMLButtonElement>(
          '[aria-label="Tela cheia"], [aria-label="Sair da tela cheia"]',
        )!;
      expect(button().getAttribute('aria-pressed')).toBe('false');
      button().click();
      expect(fs.request).toHaveBeenCalledTimes(1);
      fs.enter(el);
      fixture.detectChanges();
      expect(button().getAttribute('aria-label')).toBe('Sair da tela cheia');
      expect(button().getAttribute('aria-pressed')).toBe('true');
      button().click();
      expect(fs.exit).toHaveBeenCalledTimes(1);
      fs.leave();
      fixture.detectChanges();
      expect(button().getAttribute('aria-label')).toBe('Tela cheia');
    });
  });
});

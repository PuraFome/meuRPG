import { ComponentFixture, TestBed } from '@angular/core/testing';

import { MapPointKind } from '../../../../gen/meurpg/maps/v1/maps_pb';
import { DEFAULT_SETTINGS, type PaintSettings } from '../../../core/maps/paint-tools';
import { EditorBar, type EditorMode } from './editor-bar';

describe('EditorBar', () => {
  let fixture: ComponentFixture<EditorBar>;
  let el: HTMLElement;
  let modes: EditorMode[];
  let settings: PaintSettings[];
  let kinds: MapPointKind[];

  function setup(mode: EditorMode, canPaint = true, current: PaintSettings = DEFAULT_SETTINGS) {
    modes = [];
    settings = [];
    kinds = [];
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({});
    fixture = TestBed.createComponent(EditorBar);
    fixture.componentRef.setInput('mode', mode);
    fixture.componentRef.setInput('settings', current);
    fixture.componentRef.setInput('canPaint', canPaint);
    fixture.componentInstance.modeChange.subscribe((m) => modes.push(m));
    fixture.componentInstance.settingsChange.subscribe((s) => settings.push(s));
    fixture.componentInstance.toggleKind.subscribe((k) => kinds.push(k));
    fixture.detectChanges();
    el = fixture.nativeElement;
  }

  const button = (text: string) => Array.from(el.querySelectorAll<HTMLElement>('button')).find((b) => b.textContent?.trim().endsWith(text))!;
  const radio = (text: string) => Array.from(el.querySelectorAll<HTMLElement>('[role="radio"]')).find((b) => b.textContent?.trim().endsWith(text))!;

  it('says which mode it is in with a check first, and changes it', () => {
    setup('points');
    expect(radio('Pontos').getAttribute('aria-checked')).toBe('true');
    expect(radio('Pontos').textContent).toContain('check');
    radio('Pintar').click();
    expect(modes).toEqual(['paint']);
  });

  it('adds the three new kinds beside the three it always had', () => {
    setup('points');
    const names = Array.from(el.querySelectorAll('[aria-labelledby="bar-add"] button')).map((b) => b.textContent?.replace(/\s+/g, ' ').trim());
    expect(names).toHaveLength(6);
    ['Batalha', 'Submapa', 'Cena de RP', 'Luz', 'Armadilha', 'Tesouro'].forEach((n, i) => expect(names[i]?.endsWith(n), n).toBe(true));
    button('Armadilha').click();
    expect(kinds).toEqual([MapPointKind.TRAP]);
  });

  it('has icon buttons with names for the token and the zoom', () => {
    setup('points');
    for (const name of ['Adicionar token', 'Reduzir', 'Ampliar', 'Ajustar à tela']) {
      expect(el.querySelector(`button[aria-label="${name}"]`), name).not.toBeNull();
    }
  });

  it('offers the five tools, "Apagar" and the brush when painting; the chosen tool is checked and pressed', () => {
    setup('paint');
    for (const name of ['Terreno difícil', 'Parede', 'Cobertura', 'Luz', 'Porta', 'Apagar']) {
      expect(button(name), name).toBeTruthy();
    }
    expect(button('Terreno difícil').getAttribute('aria-pressed')).toBe('true');
    expect(button('Terreno difícil').textContent).toContain('check');
    button('Parede').click();
    expect(settings.at(-1)?.tool).toBe('wall');
    button('Apagar').click();
    expect(settings.at(-1)?.erase).toBe(true);
    radio('3×3').click();
    expect(settings.at(-1)?.brush).toBe(3);
  });

  it('opens a second line for the degree of the cover, and one for the light', () => {
    setup('paint', true, { ...DEFAULT_SETTINGS, tool: 'cover' });
    expect(el.textContent).toContain('Cobertura pintada');
    radio('Três quartos').click();
    expect(settings.at(-1)?.cover).toBe(2);
    setup('paint', true, { ...DEFAULT_SETTINGS, tool: 'light' });
    expect(el.textContent).toContain('Luz pintada');
    radio('Escuro').click();
    expect(settings.at(-1)?.light).toBe(1);
    setup('paint', true, { ...DEFAULT_SETTINGS, tool: 'wall' });
    expect(el.textContent).not.toContain('Cobertura pintada');
  });

  it('opens a second line for the kind of door: each kind with its own mark, checked when chosen, and "Tirar a porta"', () => {
    setup('paint', true, { ...DEFAULT_SETTINGS, tool: 'door', door: 3 });
    expect(el.textContent).toContain('Tipo de porta');
    const names = Array.from(el.querySelectorAll('[aria-labelledby="bar-door"] button')).map((b) => b.textContent?.replace(/\s+/g, ' ').trim());
    expect(names.map((n) => n?.replace('check', '').trim())).toEqual(['Fechada', 'Aberta', 'Trancada', 'Grade', 'Secreta']);
    // One mark each, in the drawing the map uses.
    expect(el.querySelectorAll('[aria-labelledby="bar-door"] app-door-mark')).toHaveLength(5);
    expect(button('Trancada').getAttribute('aria-pressed')).toBe('true');
    expect(button('Trancada').textContent).toContain('check');
    button('Grade').click();
    expect(settings.at(-1)).toMatchObject({ tool: 'door', door: 4, erase: false });
    button('Tirar a porta').click();
    expect(settings.at(-1)?.erase).toBe(true);
  });

  it('cannot paint without a grid: the tools say so (aria-disabled, dashed) and change nothing', () => {
    setup('paint', false);
    const wall = button('Parede');
    expect(wall.getAttribute('aria-disabled')).toBe('true');
    expect(wall.getAttribute('aria-describedby')).toBe('bar-why');
    expect(wall.classList).toContain('mr-button--off');
    wall.click();
    expect(settings).toEqual([]);
  });
});

import { ComponentFixture, TestBed } from '@angular/core/testing';

import { mapPoint, mapToken } from '../../core/maps/maps-testing';
import { MapPointsList, pointSub } from './map-points-list';
import { MapTokensList, TokenToggle, tokenSub } from './map-tokens-list';

describe('MapPointsList', () => {
  let fixture: ComponentFixture<MapPointsList>;
  let el: HTMLElement;

  beforeEach(() => {
    fixture = TestBed.createComponent(MapPointsList);
    fixture.componentRef.setInput('points', [
      mapPoint('a', 'Taverna do Javali', { revealed: true }),
      mapPoint('b', 'Covil dos goblins', { revealed: false, kind: 2, targetMap: { id: 'm', name: 'Covil' } as never }),
    ]);
    fixture.detectChanges();
    el = fixture.nativeElement;
  });

  it('says each state in words and offers the opposite action, naming the point', () => {
    const rows = Array.from(el.querySelectorAll('li'));
    expect(rows[0].textContent).toContain('Revelado');
    expect(rows[0].querySelector('button')?.getAttribute('aria-label')).toBe('Esconder Taverna do Javali');
    expect(rows[1].textContent).toContain('Escondido');
    expect(rows[1].querySelector('button')?.getAttribute('aria-label')).toBe(
      'Revelar aos jogadores Covil dos goblins',
    );
  });

  it('asks for the state the button names', () => {
    const asked: boolean[] = [];
    fixture.componentInstance.toggle.subscribe((t) => asked.push(t.revealed));
    el.querySelectorAll('button')[0].click();
    el.querySelectorAll('button')[1].click();
    expect(asked).toEqual([false, true]);
  });

  describe('RP scene points in the live session', () => {
    function withScenes(open: string | null | undefined) {
      const f = TestBed.createComponent(MapPointsList);
      f.componentRef.setInput('points', [
        mapPoint('s1', 'A carroça tombada', { revealed: true, sceneActions: [{ id: 'a' } as never] }),
        mapPoint('s2', 'Vau do riacho', { revealed: true }),
        mapPoint('b', 'Emboscada', { kind: 1, revealed: true }),
      ]);
      f.componentRef.setInput('openScenePointId', open);
      f.detectChanges();
      return f;
    }
    const scenes = (f: ComponentFixture<MapPointsList>) => Array.from(f.nativeElement.querySelectorAll('.row__scene'), (e) => (e as HTMLElement).textContent?.replace(/\s+/g, ' ').trim());

    it('offers nothing where there is no session', () => {
      expect(withScenes(undefined).nativeElement.querySelector('.row__scene')).toBeNull();
    });

    it('offers "Abrir cena" on every scene, even one with no actions (question 63)', () => {
      const f = withScenes(null);
      expect(scenes(f)).toEqual(['chat_bubble_outlineAbrir cena', 'chat_bubble_outlineAbrir cena']);
      const asked: string[] = [];
      f.componentInstance.openScene.subscribe((p) => asked.push(p.id));
      f.nativeElement.querySelector('.row__scene button').click();
      expect(asked).toEqual(['s1']);
      expect(f.nativeElement.querySelector('.row__scene button').getAttribute('aria-label')).toBe('Abrir cena A carroça tombada');
    });

    it('offers "Trocar para esta cena" when another is open, and says when it is this one', () => {
      expect(scenes(withScenes('s2'))[0]).toContain('Trocar para esta cena');
      expect(scenes(withScenes('s1'))[0]).toBe('castCena aberta agora');
    });
  });

  it('waits on the button whose call is in flight', () => {
    fixture.componentRef.setInput('pendingId', 'a');
    fixture.detectChanges();
    expect(el.querySelectorAll('button')[0].disabled).toBe(true);
    expect(el.querySelectorAll('button')[1].disabled).toBe(false);
  });

  it('names the target of a Submapa under its name', () => {
    expect(pointSub(mapPoint('x', 'Torre', { kind: 2, targetMap: { id: 'm', name: 'Torre de Mirathel' } as never }))).toBe(
      'Submapa: Torre de Mirathel',
    );
    expect(pointSub(mapPoint('y', 'Emboscada', { kind: 1 }))).toBe('Batalha');
  });
});

describe('MapTokensList', () => {
  const info = new Map([['p', { classSummary: 'Mago 3', playerName: 'Vinicius' }]]);

  it('says "Mago 3, de Vinicius" for a player and "NPC, inimigo" for an enemy', () => {
    expect(tokenSub(mapToken('p', 'Pensantus'), info)).toBe('Mago 3, de Vinicius');
    expect(tokenSub(mapToken('e', 'Capitão', { kind: 2 }), info)).toBe('NPC, inimigo');
    expect(tokenSub(mapToken('x', 'Sem info'), new Map())).toBe('Personagem de jogador');
  });

  it('shows Visível or Escondido and asks for the opposite', () => {
    const fixture = TestBed.createComponent(MapTokensList);
    fixture.componentRef.setInput('tokens', [mapToken('p', 'Pensantus'), mapToken('e', 'Capitão', { kind: 2, hidden: true })]);
    fixture.componentRef.setInput('info', info);
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const asked: TokenToggle[] = [];
    fixture.componentInstance.toggle.subscribe((t) => asked.push(t));
    expect(el.textContent).toContain('Visível');
    expect(el.textContent).toContain('Escondido');
    const buttons = el.querySelectorAll('button');
    expect(buttons[0].getAttribute('aria-label')).toBe('Esconder Pensantus');
    expect(buttons[1].getAttribute('aria-label')).toBe('Revelar aos jogadores Capitão');
    buttons[1].click();
    expect(asked[0].hidden).toBe(false);
    expect(asked[0].token.characterId).toBe('e');
  });
});

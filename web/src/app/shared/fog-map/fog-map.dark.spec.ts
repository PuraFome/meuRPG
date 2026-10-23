import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { decodeVision } from '../../core/maps/vision';
import { visionResponse } from '../../core/maps/vision-testing';
import { mapToken } from '../../core/maps/maps-testing';
import { MapView } from '../map-view/map-view';
import { settle } from '../../../test-settle';
import { FogMap } from './fog-map';

describe('FogMap: characters in the dark (B3)', () => {
  async function create(dark: ReadonlySet<string>, chosen: string | null = null) {
    const fixture = TestBed.createComponent(FogMap);
    fixture.componentRef.setInput('imageWidth', 960);
    fixture.componentRef.setInput('imageHeight', 640);
    fixture.componentRef.setInput('vision', decodeVision(visionResponse(['BBBB', 'BBBB'])));
    fixture.componentRef.setInput('isMaster', true);
    fixture.componentRef.setInput('tokens', [
      mapToken('c-brisa', 'Brisa', { xBp: 2000, yBp: 2000 }),
      mapToken('c-bilbo', 'bilbo', { xBp: 7000, yBp: 2000 }),
    ]);
    fixture.componentRef.setInput('dark', dark);
    fixture.componentRef.setInput('darkChosen', chosen);
    await settle(fixture, 2);
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('rings the characters in the dark and labels the chosen one', async () => {
    const { el } = await create(new Set(['c-brisa']), 'c-brisa');
    expect(el.querySelectorAll('.ring')).toHaveLength(1);
    expect(el.querySelector('.label')?.textContent?.trim()).toBe('Brisa · no escuro');
  });

  it('draws nothing of it when nobody is in the dark', async () => {
    const { el } = await create(new Set());
    expect(el.querySelector('app-dark-marks')).toBeNull();
  });

  it('names the mark in the legend, once, when someone is in the dark', async () => {
    const { el } = await create(new Set(['c-brisa']));
    expect(el.textContent).toContain('No escuro');
  });

  it('centers the map on the character', async () => {
    const { fixture, el } = await create(new Set(['c-brisa']));
    const view = fixture.debugElement.query((d) => d.componentInstance instanceof MapView)
      .componentInstance as MapView;
    const focus = vi.spyOn(view, 'focusOn');
    fixture.componentInstance.showCharacter('c-brisa');
    expect(focus).toHaveBeenCalledWith({ xBp: 2000, yBp: 2000 });
    expect(el).toBeTruthy();
  });
});

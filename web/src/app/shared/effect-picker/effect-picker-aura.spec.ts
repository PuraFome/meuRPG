import { ComponentFixture, TestBed } from '@angular/core/testing';

import { catalog } from '../../core/content/content-testing';
import { auraMenu } from '../../core/content/content-testing-menu-aura';
import { type EffectDraft, draftToEffect, emptyEffect } from '../../core/content/effect-draft';
import { EffectPicker } from './effect-picker';

const BASE = 'table_feat.effects[0]';

describe('EffectPicker: a feature aura (#579)', () => {
  const emitted: EffectDraft[] = [];
  let fixture: ComponentFixture<EffectPicker>;
  let el: HTMLElement;

  function setup(effect: EffectDraft) {
    emitted.length = 0;
    TestBed.resetTestingModule();
    fixture = TestBed.createComponent(EffectPicker);
    fixture.componentRef.setInput('effect', effect);
    fixture.componentRef.setInput('menu', auraMenu());
    fixture.componentRef.setInput('catalog', catalog());
    fixture.componentRef.setInput('basePath', BASE);
    fixture.componentRef.setInput('siblings', [effect]);
    fixture.componentRef.setInput('issuesOf', () => []);
    fixture.componentInstance.effectChange.subscribe((e) => emitted.push(e));
    fixture.detectChanges();
    el = fixture.nativeElement as HTMLElement;
  }
  const text = (e: Element) => (e.textContent ?? '').replace(/\s+/g, ' ');

  it('starts as an aura of 3 m for allies, with no switch and no duration to set', () => {
    setup(emptyEffect('aura'));
    expect(draftToEffect(emptyEffect('aura'), auraMenu())).toMatchObject({ type: 'aura' });
    expect(text(el)).toContain('Vale em todo combate');
    expect(text(el)).not.toContain('Deixa um efeito nos alvos');
    expect(text(el)).not.toContain('Em volta de quem conjura');
    const radius = el.querySelector<HTMLInputElement>(`[data-field="${BASE}.aura.aura.radius_ft"]`);
    expect(radius?.value).toBe('3');
  });

  it('is not saved while no bonus is in it: the block starts with no row', () => {
    expect(emptyEffect('aura').aura?.rows).toEqual([]);
  });
});

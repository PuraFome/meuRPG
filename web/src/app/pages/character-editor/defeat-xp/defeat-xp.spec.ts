import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl } from '@angular/forms';

import type { ChallengeRatingVm } from '../character-editor.types';
import { DefeatXp } from './defeat-xp';

const nbsp = ' ';
const TABLE: ChallengeRatingVm[] = [
  { rating: '0', xp: 10 },
  { rating: '1/8', xp: 25 },
  { rating: '1/4', xp: 50 },
  { rating: '1/2', xp: 100 },
  { rating: '1', xp: 200 },
  { rating: '2', xp: 450 },
];

@Component({
  imports: [DefeatXp],
  template: `<app-defeat-xp [rating]="rating" [xp]="xp" [table]="table" />`,
})
class Host {
  rating = new FormControl('1/4', { nonNullable: true });
  xp = new FormControl(50, { nonNullable: true });
  table = TABLE;
}

describe('DefeatXp (E7-11)', () => {
  function setup(rating = '1/4', xp = 50) {
    TestBed.configureTestingModule({ imports: [Host] });
    const fixture = TestBed.createComponent(Host);
    fixture.componentInstance.rating.setValue(rating);
    fixture.componentInstance.xp.setValue(xp);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    const host = fixture.componentInstance;
    const component = fixture.debugElement.children[0].componentInstance as DefeatXp;
    return { fixture, el, host, component };
  }

  const xpInput = (el: HTMLElement) => el.querySelector<HTMLInputElement>('input[type="number"]')!;
  const hint = (el: HTMLElement) => el.querySelector('#xp-hint')!.textContent!.trim();
  const useButton = (el: HTMLElement) =>
    Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.includes('Usar'));

  /** What choosing an ND in the list does: the select's `selectionChange` (only a person causes it). */
  function choose(fixture: ReturnType<typeof setup>['fixture'], host: Host, rating: string) {
    host.rating.setValue(rating);
    (fixture.debugElement.children[0].componentInstance as { pick(): void }).pick();
    fixture.detectChanges();
  }

  it('shows the ND, the XP and what each one says', async () => {
    const { fixture, el } = setup();
    // The select reads its selected option a microtask after it renders.
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.textContent).toContain('Nível de desafio (ND)');
    expect(el.querySelector('mat-select')?.textContent).toContain('ND 1/4');
    expect(xpInput(el).value).toBe('50');
    expect(el.textContent).toContain('Escolher o ND preenche o XP. Você pode digitar outro valor.');
    expect(hint(el)).toBe(`Da tabela: 50${nbsp}XP. O mestre pode digitar outro valor.`);
    expect(useButton(el)).toBeUndefined();
  });

  it('refills the XP from the table when the ND changes, even over a typed value', () => {
    const { fixture, el, host } = setup();
    host.xp.setValue(70); // the master typed another value
    fixture.detectChanges();
    expect(host.xp.value).toBe(70);

    choose(fixture, host, '1/2');
    expect(host.xp.value).toBe(100);
    expect(xpInput(el).value).toBe('100');
    expect(hint(el)).toBe(`Da tabela: 100${nbsp}XP. O mestre pode digitar outro valor.`);
  });

  it('keeps a typed value while the ND does not change', () => {
    const { fixture, el, host } = setup();
    xpInput(el).value = '70';
    xpInput(el).dispatchEvent(new Event('input'));
    fixture.detectChanges();

    expect(host.xp.value).toBe(70);
    expect(host.rating.value).toBe('1/4');
    expect(hint(el)).toBe(`Da tabela: 50${nbsp}XP. Você digitou outro valor.`);
  });

  it('does not touch the XP when a saved sheet loads (only a person choosing refills it)', () => {
    const { host } = setup('1', 120);
    expect(host.xp.value).toBe(120);
  });

  it('offers "Usar 50 XP" only when the XP is not the table\'s, and puts the table\'s back', () => {
    const { fixture, el, host } = setup();
    host.xp.setValue(0);
    fixture.detectChanges();
    expect(hint(el)).toBe(`Este NPC não dá XP. Da tabela: 50${nbsp}XP.`);
    const button = useButton(el)!;
    expect(button.textContent?.trim()).toBe(`Usar 50${nbsp}XP`);

    button.click();
    fixture.detectChanges();
    expect(host.xp.value).toBe(50);
    // The button that had the focus is gone: the focus goes to the field.
    expect(document.activeElement).toBe(xpInput(el));
    expect(useButton(el)).toBeUndefined();
  });

  it('says 0 XP is a choice, and accepts it', () => {
    const { fixture, el, host } = setup();
    xpInput(el).value = '0';
    xpInput(el).dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(host.xp.valid).toBe(true);
    expect(hint(el)).toContain('Este NPC não dá XP.');
  });

  it('writes a sheet with no ND as "Sem ND", and asks for one', async () => {
    const { fixture, el } = setup('', 0);
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('mat-select')?.textContent).toContain('Sem ND');
    expect(hint(el)).toBe('Escolha o ND para ver o XP da tabela.');
    expect(useButton(el)).toBeUndefined();
  });

  it("lists the rows with the XP beside each, in the table's order", () => {
    const { component } = setup();
    expect(
      TABLE.map((row) => `${component['ratingText'](row)} ${component['xpText'](row)}`),
    ).toEqual([
      `ND 0 10${nbsp}XP`,
      `ND 1/8 25${nbsp}XP`,
      `ND 1/4 50${nbsp}XP`,
      `ND 1/2 100${nbsp}XP`,
      `ND 1 200${nbsp}XP`,
      `ND 2 450${nbsp}XP`,
    ]);
  });

  it('writes the "mais … opções" line in the singular and the plural', () => {
    const { component } = setup();
    component['remaining'].set(28);
    expect(component['moreText']()).toBe('mais 28 opções, até ND 30');
    component['remaining'].set(1);
    expect(component['moreText']()).toBe('mais 1 opção, até ND 30');
  });
});

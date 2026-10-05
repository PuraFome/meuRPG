import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

import { XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { type TreasureToConvert, TreasureToConvertSchema } from '../../../gen/meurpg/progression/v1/progression_pb';
import type { LoadState } from '../../core/progression/experience-store';
import { TreasureStrip } from './treasure-strip';

const nbsp = ' ';

function treasure(i: number, name: string, valuePo: number, finders: string[]) {
  return create(TreasureToConvertSchema, {
    pointId: `t${i}`,
    name,
    valuePo,
    foundAt: timestampFromDate(new Date(2026, 9, 4, 21, 40)),
    foundBy: finders.map((f) => ({ characterId: f, characterName: f })),
  });
}
const THREE = [
  treasure(1, 'Baú de moedas', 250, ['Brisa']),
  treasure(2, 'Bolsa do capitão', 120, ['Toren']),
  treasure(3, 'Ídolo de prata', 50, ['Pensantus', 'Sálvia']),
];

@Component({
  imports: [TreasureStrip],
  template: `<app-treasure-strip
    [treasures]="treasures"
    [total]="total"
    [mode]="mode"
    [state]="state"
    [button]="button"
    (town)="towns = towns + 1"
    (retry)="retries = retries + 1"
  />`,
})
class Host {
  treasures: readonly TreasureToConvert[] = THREE;
  total = 3;
  mode = XpMode.GOLD;
  state: LoadState = 'ready';
  button = false;
  towns = 0;
  retries = 0;
}

describe('TreasureStrip (E9-09)', () => {
  function setup(over: Partial<Host> = {}) {
    const fixture = TestBed.createComponent(Host);
    Object.assign(fixture.componentInstance, over);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement, host: fixture.componentInstance };
  }
  const text = (el: HTMLElement, s: string) => el.querySelector(s)?.textContent?.replace(/[ \t\r\n]+/g, ' ').trim();

  it('in a campaign by gold: "Encontrado, ainda não convertido", the count, the PO and each find', () => {
    const { el } = setup();
    expect(text(el, '.strip__label')).toBe('Encontrado, ainda não convertido');
    expect(text(el, '.strip__big')).toBe(`3\u00a0tesouros · 420${nbsp}PO`);
    expect(Array.from(el.querySelectorAll('.strip__list li'), (li) => li.textContent)).toEqual([
      `Baú de moedas, 250${nbsp}PO, de Brisa`,
      `Bolsa do capitão, 120${nbsp}PO, de Toren`,
      `Ídolo de prata, 50${nbsp}PO, de Pensantus e Sálvia`,
    ]);
    // The artboard of the panel has no button in the strip: "Voltar à cidade" is in the heading.
    expect(el.querySelector('button')).toBeNull();
  });

  it('has the outlined "Voltar à cidade" in the "Dar XP" strip, and says so', () => {
    const { el, host, fixture } = setup({ button: true });
    const go = el.querySelector<HTMLButtonElement>('.strip__go')!;
    expect(go.textContent).toContain('Voltar à cidade');
    expect(go.classList.contains('mat-mdc-outlined-button')).toBe(true);
    go.click();
    fixture.detectChanges();
    expect(host.towns).toBe(1);
  });

  it('invites the next find when nothing waits, in a campaign by gold', () => {
    const { el } = setup({ treasures: [], total: 0 });
    expect(text(el, '.strip__line')).toBe('Nenhum tesouro esperando. Os que o grupo encontrar aparecem aqui.');
    expect(el.querySelector('.strip__big')).toBeNull();
  });

  it('names the first five finds and counts the rest, and says when the server holds more (100 at most)', () => {
    const many = Array.from({ length: 7 }, (_, i) => treasure(i, `Tesouro ${i + 1}`, 10, ['Brisa']));
    const { el } = setup({ treasures: many, total: 130 });
    expect(el.querySelectorAll('.strip__list li')).toHaveLength(5);
    const more = Array.from(el.querySelectorAll('.strip__more'), (p) => p.textContent);
    expect(more).toEqual(['e mais 2', 'Há mais 123\u00a0tesouros encontrados, que ficam para depois.']);
    expect(text(el, '.strip__big')).toBe(`7\u00a0tesouros · 70${nbsp}PO`);
  });

  it('in a campaign by enemies: "Tesouro encontrado", the line why, and no button', () => {
    const { el } = setup({ mode: XpMode.ENEMIES, treasures: [THREE[0]], total: 1, button: true });
    expect(text(el, '.strip__label')).toBe('Tesouro encontrado');
    expect(text(el, '.strip__big')).toBe(`1\u00a0tesouro · 250${nbsp}PO`);
    expect(text(el, '.strip__why')).toBe(
      'Esta campanha dá XP por inimigos, então o tesouro não vira XP. Ele aparece no resumo de cada sessão.',
    );
    expect(el.querySelector('button')).toBeNull();
  });

  it('is not there at all for a campaign by enemies with nothing found, or by milestones', () => {
    expect(setup({ mode: XpMode.ENEMIES, treasures: [], total: 0 }).el.querySelector('.strip')).toBeNull();
    TestBed.resetTestingModule();
    expect(setup({ mode: XpMode.MILESTONES }).el.querySelector('.strip')).toBeNull();
  });

  it('says while it reads, and offers to try again when it cannot', () => {
    expect(text(setup({ treasures: [], total: 0, state: 'loading' }).el, '.strip__line')).toBe('Lendo os tesouros encontrados...');
    TestBed.resetTestingModule();
    const { el, host, fixture } = setup({ treasures: [], total: 0, state: 'error' });
    expect(text(el, '.strip__line')).toContain('Não foi possível ler os tesouros.');
    el.querySelector<HTMLButtonElement>('.strip__retry')!.click();
    fixture.detectChanges();
    expect(host.retries).toBe(1);
  });
});

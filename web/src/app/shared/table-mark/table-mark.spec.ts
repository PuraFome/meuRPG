import { TestBed } from '@angular/core/testing';

import { TableMark } from './table-mark';

describe('TableMark', () => {
  function render(inputs: Record<string, boolean>) {
    const fixture = TestBed.createComponent(TableMark);
    for (const [k, v] of Object.entries(inputs)) fixture.componentRef.setInput(k, v);
    fixture.detectChanges();
    return Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('.mr-tag')).map((t) => t.textContent?.replace(/menu_book|inventory_2|visibility_off/g, '').trim());
  }

  it('says "Da mesa" for the table\'s entries, in words as well as the icon', () => {
    expect(render({ fromTable: true })).toEqual(['Da mesa']);
    expect(render({})).toEqual([]);
  });

  it('says what the master retired or switched off, never by colour alone', () => {
    expect(render({ fromTable: true, archived: true })).toEqual(['Da mesa', 'Arquivada']);
    expect(render({ off: true })).toEqual(['Desligada para os jogadores']);
  });

  it('writes the masculine for an antecedente', () => {
    expect(render({ archived: true, masculine: true })).toEqual(['Arquivado']);
  });
});

import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import type { FeatCatalogVm } from '../character-editor.types';
import { FeatsField } from './feats-field';

const CATALOG: FeatCatalogVm = {
  featsAllowed: true,
  options: [
    { key: 'feat:grappler', namePt: 'Agarrador', fromTable: false, off: false },
    { key: 'feat:sortudo@mesa', namePt: 'Sortudo', fromTable: true, off: false },
    { key: 'feat:mente@mesa', namePt: 'Mente afiada', fromTable: true, off: true },
  ],
};

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FeatsField],
  template: `<app-feats-field
    [featKeys]="featKeys"
    [featSlots]="featSlots"
    [catalog]="catalog"
    [issues]="issues"
    (added)="added.push($event)"
    (removed)="removed.push($event)"
  />`,
})
class Host {
  featKeys: string[] = ['feat:grappler', 'feat:sortudo@mesa'];
  featSlots: Record<string, string> = {
    'feat:grappler': 'feature:wizard-ability-score-improvement-1',
  };
  catalog: FeatCatalogVm | undefined = CATALOG;
  issues: Record<string, string> = {};
  added: string[] = [];
  removed: string[] = [];
}

describe('FeatsField (MR-025: the master gives and takes feats in the sheet editor)', () => {
  function setup(patch: Partial<Host> = {}) {
    TestBed.configureTestingModule({ imports: [Host] });
    const fixture = TestBed.createComponent(Host);
    Object.assign(fixture.componentInstance, patch);
    fixture.detectChanges();
    const el = fixture.nativeElement as HTMLElement;
    return { fixture, host: fixture.componentInstance, el };
  }

  const rows = (el: HTMLElement) => Array.from(el.querySelectorAll('li.feat'));

  it('lists the feats of the sheet with their names and where each came from', () => {
    const { el } = setup();
    const lines = rows(el).map((r) => r.textContent!.replace(/\s+/g, ' '));
    expect(lines).toHaveLength(2);
    expect(lines[0]).toContain('Agarrador');
    expect(lines[0]).toContain('no lugar de um aumento de habilidade');
    expect(lines[1]).toContain('Sortudo');
    expect(lines[1]).toContain('Da mesa');
    expect(lines[1]).toContain('dado pelo mestre');
  });

  it('says that the sheet has no feat', () => {
    const { el } = setup({ featKeys: [], featSlots: {} });
    expect(rows(el)).toHaveLength(0);
    expect(el.textContent).toContain('Esta ficha não tem talentos.');
  });

  it('removes a feat by name, with an accessible label, and warns that the improvement comes back', () => {
    const { fixture, host, el } = setup();
    const button = rows(el)[0].querySelector('button')!;
    expect(button.textContent).toContain('Remover');
    expect(button.textContent).toContain('o talento Agarrador');
    button.click();
    fixture.detectChanges();
    expect(host.removed).toEqual(['feat:grappler']);
    expect(el.textContent).toContain('traz o aumento de volta');
  });

  it('offers the feats the sheet does not have, the switched-off ones included, and emits the pick', () => {
    const { fixture, host, el } = setup();
    const field = fixture.debugElement.children[0].componentInstance as { add(key: string): void };
    expect(el.textContent).toContain('Adicionar talento');
    field.add('feat:mente@mesa');
    expect(host.added).toEqual(['feat:mente@mesa']);
    field.add('');
    expect(host.added).toEqual(['feat:mente@mesa']);
  });

  it('shows the server warning about a prerequisite without blocking the feat', () => {
    const { el } = setup({
      issues: { 'feat:grappler': 'O personagem não cumpre mais o pré-requisito de Agarrador.' },
    });
    const warning = rows(el)[0].querySelector('.feat__warning')!;
    expect(warning.textContent).toContain('pré-requisito de Agarrador');
    expect(warning.textContent).toContain('Você pode manter assim.');
    expect(rows(el)[1].querySelector('.feat__warning')).toBeNull();
  });

  it('says the table rule is off and still lets the master add', () => {
    const { el } = setup({ catalog: { ...CATALOG, featsAllowed: false } });
    expect(el.textContent).toContain('“Talentos” está desligada');
    expect(el.textContent).toContain('Adicionar talento');
  });

  it('says the list could not be read, and still lists the keys the sheet has', () => {
    const { el } = setup({ catalog: undefined });
    expect(el.textContent).toContain('não carregou');
    expect(rows(el)[0].textContent).toContain('feat:grappler');
    expect(el.textContent).not.toContain('Adicionar talento');
  });
});

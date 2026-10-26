import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  TableContentKind,
  TableContentRefusalSchema,
  TableContentViolationSchema,
  TableSubclassSchema,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { TableContentClient } from '../../../core/content/content-client';
import { catalog, classDefaults, entry } from '../../../core/content/content-testing';
import { menu } from '../../../core/content/content-testing-menu';
import { settle } from '../../../../test-settle';
import { SubclassEditor } from './subclass-editor';

vi.setConfig({ testTimeout: 30_000 });

// The switch "As magias da subclasse entram na lista da classe, para escolher" (MR-025): shown when the subclass has spells,
// off by default, and the request it builds.
describe('SubclassEditor: the spells that join the class list', () => {
  const save = vi.fn();
  const SWITCH = '[data-field="table_subclass.spells_expanded"] [role="switch"]';

  const tinta = (over: { expanded?: boolean; spells?: boolean } = {}) =>
    entry(TableContentKind.SUBCLASS, 'Tradição da Tinta', {
      body: {
        case: 'tableSubclass',
        value: create(TableSubclassSchema, {
          namePt: 'Tradição da Tinta',
          classKey: 'class:wizard',
          alwaysPrepared: over.spells === false ? [] : [{ classLevel: 3, spellKey: 'spell:light' }],
          spellsExpanded: over.expanded ?? false,
        }),
      },
    });

  function setup(e: ReturnType<typeof tinta>) {
    save.mockReset();
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: TableContentClient, useValue: { save } }],
    });
    const fixture = TestBed.createComponent(SubclassEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('catalog', catalog());
    fixture.componentRef.setInput('menu', menu());
    fixture.componentRef.setInput('defaults', classDefaults());
    fixture.componentRef.setInput('entry', e);
    fixture.componentRef.setInput('parentKey', '');
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const text = (el: Element) => (el.textContent ?? '').replace(/\s+/g, ' ');
  const toggle = (el: HTMLElement) => el.querySelector<HTMLButtonElement>(SWITCH)!;
  const saveSubclass = async (fixture: ReturnType<typeof setup>['fixture'], el: HTMLElement) => {
    save.mockResolvedValue({ entry: tinta(), affected: [] });
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes('Salvar subclasse'))!
      .click();
    await settle(fixture, 3);
    return save.mock.calls[0][2].value;
  };

  it('is off when the subclass gives its spells, with the sentence that says so', () => {
    const { el } = setup(tinta());
    expect(toggle(el).getAttribute('aria-checked')).toBe('false');
    expect(text(el)).toContain('As magias da subclasse entram na lista da classe, para escolher');
    expect(text(el)).toContain('Desligado: elas são sempre preparadas.');
  });

  it('turns on, says what happens, and sends spells_expanded', async () => {
    const { fixture, el } = setup(tinta());
    toggle(el).click();
    await settle(fixture, 3);
    expect(toggle(el).getAttribute('aria-checked')).toBe('true');
    expect(text(el)).toContain(
      'A pessoa escolhe estas magias como as outras da lista, e elas contam no limite.',
    );
    const body = await saveSubclass(fixture, el);
    expect(body.spellsExpanded).toBe(true);
    expect(body.alwaysPrepared).toEqual([{ classLevel: 3, spellKey: 'spell:light' }]);
  });

  it('reads a stored subclass with the switch on, and turns it off', async () => {
    const { fixture, el } = setup(tinta({ expanded: true }));
    expect(toggle(el).getAttribute('aria-checked')).toBe('true');
    toggle(el).click();
    await settle(fixture, 3);
    const body = await saveSubclass(fixture, el);
    expect(body.spellsExpanded).toBe(false);
  });

  it('is not offered to a subclass without spells, and the request never carries it', async () => {
    const { fixture, el } = setup(tinta({ spells: false, expanded: true }));
    expect(el.querySelector(SWITCH)).toBeNull();
    const body = await saveSubclass(fixture, el);
    expect(body.spellsExpanded).toBe(false);
  });

  it('writes the refusal under the switch', async () => {
    const { fixture, el } = setup(tinta({ expanded: true }));
    save.mockRejectedValue(
      new ConnectError('refused', Code.InvalidArgument, undefined, [
        {
          desc: TableContentRefusalSchema,
          value: create(TableContentRefusalSchema, {
            violations: [
              create(TableContentViolationSchema, {
                field: 'table_subclass.spells_expanded',
                reason: 'bad_value',
              }),
            ],
          }),
        },
      ]),
    );
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes('Salvar subclasse'))!
      .click();
    await settle(fixture, 3);
    expect(text(el.querySelector('.expanded')!)).toContain(
      'Esta marca precisa de magias na subclasse. Adicione magias ou desligue.',
    );
  });
});

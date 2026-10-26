import { TestBed } from '@angular/core/testing';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import {
  TableContentKind,
  TableContentRefusalSchema,
  TableContentViolationSchema,
  TableOptionSchema,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { TableContentClient } from '../../../core/content/content-client';
import { catalog, entry } from '../../../core/content/content-testing';
import { menu } from '../../../core/content/content-testing-menu';
import { OptionEditor } from './option-editor';
import { settle } from '../../../../test-settle';

function refusal(...violations: [string, string][]) {
  return new ConnectError('refused', Code.InvalidArgument, undefined, [
    {
      desc: TableContentRefusalSchema,
      value: create(TableContentRefusalSchema, {
        violations: violations.map(([field, reason]) =>
          create(TableContentViolationSchema, { field, reason }),
        ),
      }),
    },
  ]);
}

describe('OptionEditor (MR-025)', () => {
  const save = vi.fn();
  const stored = () =>
    entry(TableContentKind.OPTION, 'Olhar do Corvo', {
      body: {
        case: 'tableOption',
        value: create(TableOptionSchema, {
          namePt: 'Olhar do Corvo',
          descPt: ['Você enxerga o que o corvo vê.'],
          set: 'invocation',
          prerequisite: {
            classLevel: { classKey: 'class:wizard', level: 5 },
            requiresPact: 'feature:pact-of-the-tome',
          },
          effects: [{ type: 'note', textPt: 'Enxerga pelos olhos de um corvo.' }],
        }),
      },
    });

  function setup(existing = true) {
    save.mockReset();
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [{ provide: TableContentClient, useValue: { save } }],
    });
    const fixture = TestBed.createComponent(OptionEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('catalog', catalog());
    fixture.componentRef.setInput('menu', menu());
    if (existing) fixture.componentRef.setInput('entry', stored());
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const text = (e: Element) => (e.textContent ?? '').replace(/\s+/g, ' ');
  const field = (el: HTMLElement, path: string) =>
    el.querySelector<HTMLElement>(`[data-field="${path}"]`)!;
  const click = (el: HTMLElement, label: string) =>
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes(label))!
      .click();

  it('draws the form from the entry: name, text, the set read-only, the class level and the pact', () => {
    const { el } = setup();
    expect((field(el, 'table_option.name_pt') as HTMLInputElement).value).toBe('Olhar do Corvo');
    expect((field(el, 'table_option.desc_pt') as HTMLTextAreaElement).value).toBe(
      'Você enxerga o que o corvo vê.',
    );
    expect(text(field(el, 'table_option.set'))).toContain('Invocação mística');
    expect(field(el, 'table_option.set').tagName).not.toBe('SELECT');
    expect(
      (field(el, 'table_option.prerequisite.class_level.level') as HTMLInputElement).value,
    ).toBe('5');
    expect(
      (field(el, 'table_option.prerequisite.requires_pact') as HTMLSelectElement).selectedOptions[0]
        .text,
    ).toBe('Pacto do Tomo');
  });

  it('writes "Como os jogadores veem" from the draft', () => {
    const { el } = setup();
    expect(text(el.querySelector('.preview')!)).toContain('Você enxerga o que o corvo vê.');
  });

  it('a new option picks the set from the menu and sends it with the whole body', async () => {
    const { fixture, el } = setup(false);
    const set = field(el, 'table_option.set') as HTMLSelectElement;
    expect(Array.from(set.options).map((o) => o.text)).toContain('Invocação mística');
    set.value = 'invocation';
    set.dispatchEvent(new Event('change'));
    const name = field(el, 'table_option.name_pt') as HTMLInputElement;
    name.value = 'Olhar do Corvo';
    name.dispatchEvent(new Event('input'));
    save.mockResolvedValue({ entry: stored(), affected: [] });
    click(el, 'Salvar opção');
    await settle(fixture, 3);
    const [, sent, body] = save.mock.calls[0];
    expect(sent).toBeNull();
    expect(body.case).toBe('tableOption');
    expect(body.value.set).toBe('invocation');
    expect(body.value.namePt).toBe('Olhar do Corvo');
    expect(body.value.prerequisite).toEqual({
      requiresOptionKeys: [],
      requiresPact: '',
      requiresSpellKey: '',
    });
  });

  it("puts the server's paths back on their inputs: the set, the level and the name", async () => {
    const { fixture, el } = setup();
    save.mockRejectedValue(
      refusal(
        ['table_option.name_pt', 'duplicate_name'],
        ['table_option.prerequisite.class_level.level', 'bad_value'],
        ['table_option.prerequisite.requires_pact', 'dangling_reference'],
      ),
    );
    click(el, 'Salvar opção');
    await settle(fixture, 3);
    expect(text(el.querySelector('[role="alert"]')!)).toContain('3 campos precisam de ajuste');
    expect(
      text(field(el, 'table_option.prerequisite.class_level.level').closest('app-text-field')!),
    ).toContain('O nível vai de 1 a 20');
    expect(document.activeElement).toBe(field(el, 'table_option.name_pt'));
  });

  it('a new option starts empty and is called "Nova opção" in the preview', () => {
    const { el } = setup(false);
    expect((field(el, 'table_option.name_pt') as HTMLInputElement).value).toBe('');
    expect(text(el.querySelector('.preview')!)).toContain('Nova opção');
  });
});

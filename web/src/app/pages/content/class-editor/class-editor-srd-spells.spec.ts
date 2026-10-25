import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';

import { Ability, ContentSchema } from '../../../../gen/meurpg/rules/v1/rules_pb';
import {
  TableClassSchema,
  TableContentKind,
  TableContentRefusalSchema,
  TableContentViolationSchema,
} from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { catalogVm } from '../../../core/content/catalog';
import { TableContentClient } from '../../../core/content/content-client';
import { classDefaults, entry } from '../../../core/content/content-testing';
import { menu } from '../../../core/content/content-testing-menu';
import { settle } from '../../../../test-settle';
import { ClassEditor } from './class-editor';

vi.setConfig({ testTimeout: 30_000 });

// The SRD spells on a class's own list (MR-025): the picker of "Conjuração", its three states and the request it builds.
describe('ClassEditor: Magias do SRD na lista da classe', () => {
  const save = vi.fn();
  const defaults = classDefaults();
  const cat = catalogVm(
    create(ContentSchema, {
      classes: [
        { key: 'class:wizard', namePt: 'Mago', subclassLevel: 2, spellcasting: { ability: 4 } },
      ],
      spells: [
        {
          key: 'spell:light',
          namePt: 'Luz',
          level: 0,
          schoolKey: 'school:evocation',
          schoolNamePt: 'Evocação',
        },
        {
          key: 'spell:cure-wounds',
          namePt: 'Curar Ferimentos',
          level: 1,
          schoolKey: 'school:evocation',
          schoolNamePt: 'Evocação',
        },
        {
          key: 'spell:aid',
          namePt: 'Auxílio',
          level: 2,
          schoolKey: 'school:abjuration',
          schoolNamePt: 'Abjuração',
        },
        {
          key: 'spell:selo@mesa',
          namePt: 'Selo do Vale',
          level: 1,
          schoolKey: 'school:abjuration',
          schoolNamePt: 'Abjuração',
        },
      ],
    }),
  );

  const apprentice = (listFrom: string, srd: string[]) =>
    entry(TableContentKind.CLASS, 'Aprendiz', {
      body: {
        case: 'tableClass',
        value: create(TableClassSchema, {
          namePt: 'Aprendiz',
          hitDie: 8,
          savingThrows: [Ability.CONSTITUTION, Ability.INTELLIGENCE],
          skillChoose: 2,
          skillFrom: ['skill:arcana'],
          subclassLevel: 3,
          asiLevels: [4, 8, 12, 16, 19],
          casting: {
            kind: 'half',
            ability: Ability.INTELLIGENCE,
            preparation: 'prepared',
            listFrom,
            startLevel: 2,
            srdSpellKeys: srd,
          },
          levels: defaults.tables[3].rows.map((r) => ({ ...r, features: [] })),
        }),
      },
    });

  function setup(e: ReturnType<typeof apprentice>, issues: [string, string][] = []) {
    save.mockReset();
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [provideRouter([]), { provide: TableContentClient, useValue: { save } }],
    });
    const fixture = TestBed.createComponent(ClassEditor);
    fixture.componentRef.setInput('campaignId', 'camp-1');
    fixture.componentRef.setInput('catalog', cat);
    fixture.componentRef.setInput('menu', menu());
    fixture.componentRef.setInput('defaults', defaults);
    fixture.componentRef.setInput('entry', e);
    fixture.detectChanges();
    if (issues.length > 0) {
      save.mockRejectedValue(
        new ConnectError('refused', Code.InvalidArgument, undefined, [
          {
            desc: TableContentRefusalSchema,
            value: create(TableContentRefusalSchema, {
              violations: issues.map(([field, reason]) =>
                create(TableContentViolationSchema, { field, reason }),
              ),
            }),
          },
        ]),
      );
    }
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const text = (el: Element) => (el.textContent ?? '').replace(/\s+/g, ' ');
  const picker = (el: HTMLElement) =>
    el.querySelector<HTMLElement>('[data-field="table_class.casting.srd_spell_keys"]')!;
  const select = (el: HTMLElement) => picker(el).querySelector<HTMLSelectElement>('select');
  const chips = (el: HTMLElement) =>
    Array.from(picker(el).querySelectorAll('.chip')).map((c) =>
      text(c).replace('close', '').trim(),
    );
  const pick = (el: HTMLElement, key: string) => {
    const s = select(el)!;
    s.value = key;
    s.dispatchEvent(new Event('change'));
  };
  const saveClass = async (fixture: ReturnType<typeof setup>['fixture'], el: HTMLElement) => {
    save.mockResolvedValue({ entry: apprentice('', []), affected: [] });
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes('Salvar classe'))!
      .click();
    await settle(fixture, 3);
    return save.mock.calls[0][2].value;
  };

  it("is empty on a class with a list of its own: it offers the SRD spells with their circle and school, never the table's", () => {
    const { el } = setup(apprentice('', []));
    expect(text(el)).toContain('Magias do SRD na lista da classe');
    expect(chips(el)).toEqual([]);
    expect(select(el)!.disabled).toBe(false);
    expect(Array.from(select(el)!.options).map((o) => o.text.trim())).toEqual([
      '',
      'Auxílio (2º nível, Abjuração)',
      'Curar Ferimentos (1º nível, Evocação)',
      'Luz (Truque, Evocação)',
    ]);
    expect(text(el)).toContain('Elas se juntam às magias da mesa que citam esta classe.');
  });

  it('adds a spell as a chip, takes it off again, and sends the keys in the order they were added', async () => {
    const { fixture, el } = setup(apprentice('', ['spell:light']));
    expect(chips(el)).toEqual(['Luz (Truque, Evocação)']);
    pick(el, 'spell:aid');
    await settle(fixture, 3);
    pick(el, 'spell:cure-wounds');
    await settle(fixture, 3);
    expect(chips(el)).toEqual([
      'Luz (Truque, Evocação)',
      'Auxílio (2º nível, Abjuração)',
      'Curar Ferimentos (1º nível, Evocação)',
    ]);
    (
      picker(el).querySelector(
        'button[aria-label="Tirar Luz (Truque, Evocação)"]',
      ) as HTMLButtonElement
    ).click();
    await settle(fixture, 3);
    const body = await saveClass(fixture, el);
    expect(body.casting.srdSpellKeys).toEqual(['spell:aid', 'spell:cure-wounds']);
    expect(body.casting.listFrom).toBe('');
  });

  it('is off, with the reason, on a class that reuses the list of another: the chips stay, the request sends none', async () => {
    const { fixture, el } = setup(apprentice('class:wizard', ['spell:light']));
    expect(chips(el)).toEqual(['Luz (Truque, Evocação)']);
    expect(select(el)!.disabled).toBe(true);
    expect((picker(el).querySelector('.chip__x') as HTMLButtonElement).disabled).toBe(true);
    expect(text(el)).toContain(
      'Esta classe usa a lista de outra classe. Para ter magias próprias, escolha "A própria lista da classe".',
    );
    const body = await saveClass(fixture, el);
    expect(body.casting.listFrom).toBe('class:wizard');
    expect(body.casting.srdSpellKeys).toEqual([]);
  });

  it('writes the refusal of a spell under the picker, in Portuguese', async () => {
    const { fixture, el } = setup(apprentice('', ['spell:light', 'spell:aid']), [
      ['table_class.casting.srd_spell_keys[1]', 'dangling_reference'],
    ]);
    Array.from(el.querySelectorAll<HTMLButtonElement>('button'))
      .find((b) => text(b).includes('Salvar classe'))!
      .click();
    await settle(fixture, 3);
    expect(text(picker(el).parentElement!)).toContain(
      'Esta magia não está no SRD. Tire-a da lista.',
    );
  });
});

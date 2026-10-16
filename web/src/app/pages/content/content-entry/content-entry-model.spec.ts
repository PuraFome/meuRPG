import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';

import { TableContentKind } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { entry } from '../../../core/content/content-testing';
import {
  EDITABLE,
  NEW_KINDS,
  archivedDay,
  archivedWordOf,
  backLabelOf,
  entriesAfterSave,
  extrasErrorOf,
  extrasErrors,
  headingOf,
  kindTagOf,
  mergeRefreshed,
  nameResolver,
  nounPhraseOf,
  replaceEntry,
  subraceParentOf,
  updatedLine,
  usageOf,
  wireMenu,
} from './content-entry-model';

describe('the content entry model', () => {
  const race = entry(TableContentKind.RACE, 'Corujeiro');
  const feat = entry(TableContentKind.FEAT, 'Lutador de Corda');

  it('knows the words of the link and which kinds have an editor', () => {
    expect(NEW_KINDS['spell']).toBe(TableContentKind.SPELL);
    expect(NEW_KINDS['nonsense']).toBeUndefined();
    expect(EDITABLE.has(TableContentKind.FEAT)).toBe(true);
  });

  it('titles the page by the entry, by the kind of a new one, or by the table', () => {
    expect(headingOf(race, null)).toBe('Corujeiro');
    expect(headingOf(null, TableContentKind.CLASS)).toBe('Nova classe');
    expect(headingOf(null, TableContentKind.UNSPECIFIED)).toBe('Conteúdo da mesa');
    expect(headingOf(null, null)).toBe('Conteúdo da mesa');
    expect(kindTagOf(null)).toBe('');
    expect(kindTagOf(TableContentKind.RACE)).toBe('Raça da mesa');
  });

  it('writes the dates as the day only, and nothing for an entry without them', () => {
    expect(updatedLine(null)).toBe('');
    expect(archivedDay(race)).toBe('');
    const dated = {
      ...race,
      updatedAt: timestampFromDate(new Date(2026, 2, 12, 15)),
      archivedAt: timestampFromDate(new Date(2026, 3, 1, 9)),
    };
    expect(updatedLine(dated)).toBe('Atualizada em 12/03/2026');
    expect(archivedDay(dated)).toBe('01/04/2026');
  });

  it('agrees the noun and the participle with the kind', () => {
    expect(nounPhraseOf(null)).toBe('');
    expect(nounPhraseOf(race)).toBe('A raça Corujeiro');
    expect(archivedWordOf(null)).toBe('arquivada');
    expect(archivedWordOf(feat)).toBe('arquivado');
    expect(usageOf(null)).toBe('');
    expect(usageOf(race)).toContain('Corujeiro');
  });

  it('puts a saved entry in its place, or among the others by name when it is new', () => {
    const zed = entry(TableContentKind.RACE, 'Zed');
    const renamed = { ...race, namePt: 'Corujeiro 2' };
    expect(entriesAfterSave([race, zed], renamed).map((e) => e.namePt)).toEqual([
      'Corujeiro 2',
      'Zed',
    ]);
    const alpha = entry(TableContentKind.RACE, 'Alfa');
    expect(entriesAfterSave([race, zed], alpha).map((e) => e.namePt)).toEqual([
      'Alfa',
      'Corujeiro',
      'Zed',
    ]);
    expect(replaceEntry([race, zed], { ...zed, archived: true })[1].archived).toBe(true);
  });

  it('keeps what is typed in the entry being edited when another write changed it', () => {
    const changed = {
      ...race,
      revision: race.revision + 1,
      off: true,
      namePt: 'Outro',
      charactersUsing: 9,
    };
    const edited = mergeRefreshed([race], [changed], race.key, true)[0];
    expect(edited.namePt).toBe('Corujeiro');
    expect(edited.off).toBe(true);
    expect(edited.charactersUsing).toBe(9);
    expect(mergeRefreshed([race], [changed], race.key, false)[0]).toBe(changed);
    expect(mergeRefreshed([race], [{ ...race }], race.key, true)[0].namePt).toBe('Corujeiro');
    expect(mergeRefreshed([race], [changed], 'other', true)[0]).toBe(changed);
  });

  it('says which read failed, each in its words', () => {
    const ok = { status: 'fulfilled', value: 1 } as const;
    const bad = (code: Code) =>
      ({ status: 'rejected', reason: new ConnectError('x', code) }) as const;
    expect(extrasErrors([ok, ok, ok])).toEqual({ catalog: '', menu: '', defaults: '' });
    const failed = extrasErrors([bad(Code.Internal), bad(Code.Internal), bad(Code.Internal)]);
    expect(failed.catalog).not.toBe('');
    expect(failed.menu).not.toBe('');
    expect(failed.defaults).not.toBe('');
    const e = {
      catalog: '',
      menu: 'menu',
      defaults: 'defaults',
      spell: false,
      needsDefaults: false,
    };
    expect(extrasErrorOf(e)).toBe('menu');
    expect(extrasErrorOf({ ...e, spell: true })).toBe('');
    expect(extrasErrorOf({ ...e, menu: '', needsDefaults: true })).toBe('defaults');
    expect(extrasErrorOf({ ...e, catalog: 'catalog' })).toBe('catalog');
  });

  it('labels the way back, and finds the race of a subrace', () => {
    expect(backLabelOf(true, { plural: 'Raças' } as never)).toBe('Voltar para Raças');
    expect(backLabelOf(true, undefined)).toBe('Voltar para Conteúdo da mesa');
    expect(backLabelOf(false, { plural: 'Raças' } as never)).toBe('Voltar para Conteúdo da mesa');
    expect(subraceParentOf(null, 'race:x')).toBe('race:x');
    const sub = {
      ...race,
      body: { case: 'tableSubrace', value: { raceKey: 'race:own' } },
    } as never;
    expect(subraceParentOf(sub, 'race:x')).toBe('race:own');
  });

  it('names a key by the catalog, the table or the key itself, and wires the menu once both are there', () => {
    const cat = { nameOf: (k: string) => (k === 'k1' ? 'Do catálogo' : undefined) } as never;
    expect(nameResolver(cat, [race])('k1')).toBe('Do catálogo');
    expect(nameResolver(null, [race])(race.key)).toBe('Corujeiro');
    expect(nameResolver(null, [])('race:zzz')).toBe('zzz');
    const menu = { classNamePt: undefined, featOptions: undefined } as never as {
      classNamePt?: (k: string) => string;
      featOptions?: unknown;
    };
    wireMenu(menu as never, null, [feat]);
    expect(menu.featOptions).toBeUndefined();
    wireMenu(menu as never, cat, [feat, { ...feat, key: 'feat:old', archived: true }]);
    expect(menu.classNamePt?.('k1')).toBe('Do catálogo');
    expect(menu.featOptions).toEqual([{ key: feat.key, namePt: 'Lutador de Corda', hintPt: '' }]);
    expect(create).toBeDefined();
  });
});

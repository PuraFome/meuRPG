import { create } from '@bufbuild/protobuf';

import {
  TableBackgroundSchema,
  TableContentKind,
  TableFeatSchema,
  TableEntrySchema,
  TableRaceSchema,
  TableSpellSchema,
  TableSubraceSchema,
} from '../../../gen/meurpg/rules/v1/table_content_pb';
import { type EntryRead, type NameOf, readEntry } from './content-read';
import type { FeatInit } from './feat-draft';
import type { BackgroundInit, RaceInit, SubraceInit } from './feature-draft';
import type { SpellInit } from './spell-draft';

/**
 * "Como os jogadores veem" (E10-01): the editor's preview is what a player will read, so it is written by the same function
 * (`readEntry`) from the entry the form would send, effects and all. Nothing here is a second way to write a spell or a race.
 */
export type PreviewBody =
  | { readonly case: 'tableSpell'; readonly value: SpellInit }
  | { readonly case: 'tableRace'; readonly value: RaceInit }
  | { readonly case: 'tableSubrace'; readonly value: SubraceInit }
  | { readonly case: 'tableBackground'; readonly value: BackgroundInit }
  | { readonly case: 'tableFeat'; readonly value: FeatInit };

export function previewRead(body: PreviewBody, nameOf: NameOf): EntryRead {
  switch (body.case) {
    case 'tableSpell':
      return readEntry(
        create(TableEntrySchema, {
          kind: TableContentKind.SPELL,
          body: { case: 'tableSpell', value: create(TableSpellSchema, body.value) },
        }),
        nameOf,
      );
    case 'tableRace':
      return readEntry(
        create(TableEntrySchema, {
          kind: TableContentKind.RACE,
          body: { case: 'tableRace', value: create(TableRaceSchema, body.value) },
        }),
        nameOf,
      );
    case 'tableSubrace':
      return readEntry(
        create(TableEntrySchema, {
          kind: TableContentKind.SUBRACE,
          body: { case: 'tableSubrace', value: create(TableSubraceSchema, body.value) },
        }),
        nameOf,
      );
    case 'tableBackground':
      return readEntry(
        create(TableEntrySchema, {
          kind: TableContentKind.BACKGROUND,
          body: { case: 'tableBackground', value: create(TableBackgroundSchema, body.value) },
        }),
        nameOf,
      );
    case 'tableFeat':
      return readEntry(
        create(TableEntrySchema, {
          kind: TableContentKind.FEAT,
          body: { case: 'tableFeat', value: create(TableFeatSchema, body.value) },
        }),
        nameOf,
      );
  }
}

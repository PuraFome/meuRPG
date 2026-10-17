import { computed, signal } from '@angular/core';

import type { SpellCast } from '../../../../../gen/meurpg/play/v1/combat_pb';
import type { RollMode } from '../../../../../gen/meurpg/play/v1/combat_rolls_pb';
import type { SpellDetails } from '../../../../../gen/meurpg/rules/v1/rules_pb';
import type { Dealt, SlotRow } from '../../../../core/combat/cast-flow';
import { type MetamagicPicks, NO_PICKS } from '../../../../core/resources/metamagic';
import type { CastSheetData } from './cast-sheet-types';

/** What was chosen and what the cast sheet is showing, shared by the parts that read and write it. */
export class CastState {
  readonly details = signal<SpellDetails | null>(null);
  readonly detailsFailed = signal(false);
  readonly slot = signal<SlotRow | null>(null);
  readonly chosen = signal<string[]>([]);
  readonly dealt = signal<Dealt>(new Map());
  readonly typing = signal(false);
  /** The mode of a spell attack's d20 the caster picked; null leaves it to the app's suggestion for each target. */
  readonly picked = signal<RollMode | null>(null);
  readonly reason = signal('');
  readonly busy = signal(false);
  readonly error = signal('');
  readonly cast = signal<SpellCast | null>(null);
  /** The Metamagic options marked: at most one, or Empowered Spell with one other; each may ask for creatures (`picks`). */
  readonly chosenMeta = signal<string[]>([]);
  readonly picks = signal<MetamagicPicks>(NO_PICKS);
  /** Aprimorar Habilidade names the ability it is cast for; empty until picked, and for every other spell. */
  readonly ability = signal('');
  /** The picked damage type; the first of the spell's choices until the caster picks another. */
  readonly pickedType = signal('');
  readonly sculpted = signal<string[]>([]);
  /** A list spell that may hit nobody, with nobody ticked: "Conjurar" asks in place first (PM-02d state 13). */
  readonly listAsking = signal(false);
  readonly slotLevel = computed(() => this.slot()?.level ?? this.data.level);
  /** Whether the cast happened (or is being resumed): the result stage. */
  readonly done = computed(() => this.cast() !== null || !!this.data.resume);

  constructor(readonly data: CastSheetData) {}
}

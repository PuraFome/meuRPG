import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import { LevelUpRefusalReason } from '../../../../gen/meurpg/characters/v1/characters_pb';
import { Ability } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { ABILITY_LABELS, formatModifier } from '../../../core/characters/character-labels';
import { ABILITY_KEYS, type AbilityKey } from '../../../core/characters/characters.types';
import { FeatPicker } from '../../../shared/feat-picker/feat-picker';
import { ChangeRowsList } from '../change-rows/change-rows';
import { LevelUpSession } from '../level-up-session';

const WIRE: Record<AbilityKey, Ability> = {
  str: Ability.STRENGTH,
  dex: Ability.DEXTERITY,
  con: Ability.CONSTITUTION,
  int: Ability.INTELLIGENCE,
  wis: Ability.WISDOM,
  cha: Ability.CHARISMA,
};

/** What the Habilidades step lists about one ability. */
interface AbilityRow {
  readonly key: AbilityKey;
  readonly label: string;
  /** "18", or "18 → 20" once it is picked. */
  readonly score: string;
  /** "modificador +4 → +5" for a picked one, empty otherwise. */
  readonly modifier: string;
  readonly picked: boolean;
}

/** The changes the step shows under "O que muda com ...": what an ability moves, not the level's own. */
const ABILITY_DERIVED = /^(dc|attack|prepared|armor|initiative|save-|skills-|passive-)/;

/**
 * Step "Habilidades" of the guided level-up (MR-040, E8-15): the SRD's Ability Score
 * Improvement, "+2 em uma habilidade" or "+1 em duas", none above 20, with the score
 * before → after of the picked ones and "O que muda com Inteligência 20", read from the
 * preview (the browser computes no modifier). When the table allows feats (MR-025) the server lists them and the step
 * offers "Aumentar habilidades" or "Um talento" (`app-feat-picker`); with no feats listed it is the increase alone.
 */
@Component({
  selector: 'app-abilities-step',
  imports: [ChangeRowsList, FeatPicker, MatIconModule],
  templateUrl: './abilities-step.html',
  styleUrl: './abilities-step.scss',
})
export class AbilitiesStep {
  readonly s = input.required<LevelUpSession>();

  protected readonly missing = computed(() => this.s().draft.missingIn('abilities').length > 0);
  protected readonly one = computed(() => this.s().draft.abilityMode() === 'one');

  protected readonly hasFeats = computed(() => this.s().draft.hasFeats());
  protected readonly taking = computed(() => this.s().draft.taking());
  protected readonly nameOf = computed(() => {
    const names = this.s().draft.names();
    return (key: string) => names.get(key) ?? key.replace(/^[a-z-]+:/, '');
  });

  protected readonly rows = computed<AbilityRow[]>(() => {
    const s = this.s();
    const d = s.draft;
    const picked = d.pickedKeys();
    const after = s.after();
    return ABILITY_KEYS.map((key) => {
      const was = s.before.abilities.find((a) => a.ability === WIRE[key]);
      const now = after.abilities.find((a) => a.ability === WIRE[key]);
      const score = was?.score ?? 0;
      const on = picked.includes(key);
      const moved = on && was && now && now.score !== was.score;
      return {
        key,
        label: ABILITY_LABELS[key],
        score: moved ? `${was.score} → ${now.score}` : String(score),
        modifier: moved
          ? `modificador ${formatModifier(was.modifier)} → ${formatModifier(now.modifier)}`
          : '',
        picked: on,
      };
    });
  });

  /** "Inteligência 20", or "Inteligência 19 e Sabedoria 14": the abilities picked, as they will be. */
  protected readonly title = computed(() => {
    const s = this.s();
    const names = s.draft.pickedKeys().map((key) => {
      const now = s.after().abilities.find((a) => a.ability === WIRE[key]);
      return `${ABILITY_LABELS[key]} ${now?.score ?? ''}`;
    });
    return names.length === 0
      ? ''
      : new Intl.ListFormat('pt-BR', { type: 'conjunction' }).format(names);
  });

  /** The "O que muda" panel when there is nothing to show yet. */
  protected readonly noChange = computed(() => {
    const d = this.s().draft;
    if (!d.taking()) {
      return 'Escolha uma habilidade para ver o que muda.';
    }
    const feat = d.feat();
    if (!feat) {
      return 'Escolha um talento para ver o que muda.';
    }
    return feat.increase
      ? 'Escolha as habilidades para ver o que muda.'
      : 'Este talento não muda habilidades.';
  });
  protected readonly changes = computed(() =>
    this.s()
      .rows()
      .filter((r) => ABILITY_DERIVED.test(r.key)),
  );
  /** The server says the pick would take an ability past 20: the rule is its, the page only shows it. */
  protected readonly aboveTwenty = computed(() => {
    const p = this.s().preview.state();
    return !p.loading && p.refusal?.reason === LevelUpRefusalReason.ABILITY_ABOVE_20;
  });
  protected readonly constitutionPicked = computed(() =>
    this.s().draft.pickedKeys().includes('con'),
  );
}

import { Component, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import type { Observable } from 'rxjs';

import { DiceMode, DicePreference } from '../../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { MapPointKind } from '../../../../../gen/meurpg/maps/v1/maps_pb';
import type { SearchForTrapsResponse } from '../../../../../gen/meurpg/play/v1/traps_pb';
import { effectivePreference } from '../../../../core/campaigns/dice-labels';
import { rollText } from '../../../../core/combat/combat-dice';
import { newKey } from '../../../../core/connect/idempotency';
import type { MapState } from '../../../../core/maps/map-state';
import { needsTwoDice, trapErrorMessage } from '../../../../core/traps/trap-errors';
import {
  SEARCH_STEPS,
  type SearchSkills,
  resultMessage,
  searchStep,
  skillName,
  skillOptions,
} from '../../../../core/traps/trap-search';
import { type SearchDie, type SearchSkill, TrapsClient } from '../../../../core/traps/traps-client';
import { mediaQuery } from '../../../../shared/map-view/media-query';
import { RollPicker } from '../../combat/roll-picker/roll-picker';
import { SheetFrame } from '../../combat/sheet-frame/sheet-frame';
import { injectSheet, openSheet } from '../../combat/sheet-host';

/** What the page hands "Procurar armadilhas". */
export interface TrapSearchData {
  readonly campaignId: string;
  /** The character's bonuses in the two skills (the derived sheet); `null` while unknown. */
  readonly skills: SearchSkills | null;
  readonly diceMode: DiceMode;
  readonly preference: DicePreference;
  /** The map the player sees: read again after the search, so the trap found shows by name. */
  readonly state: MapState;
  /** The search is the Search action of the character's turn: it spends the action. */
  readonly inCombat: boolean;
}

/** "Procurar armadilhas": a bottom sheet on a phone, a dialog from a tablet up. It answers `true` when a search was made. */
export function openTrapSearch(dialog: MatDialog, bottomSheet: MatBottomSheet, data: TrapSearchData): Observable<boolean | undefined> {
  return openSheet<TrapSearchSheet, TrapSearchData, boolean>(dialog, bottomSheet, TrapSearchSheet, {
    data,
    ariaLabel: 'Procurar armadilhas',
    labelledBy: 'trap-search-t',
    focus: 'input[type=radio]',
  });
}

/**
 * "Procurar armadilhas" (E9-08 A to C and H, MR-035, question 71, RN-18): the player picks **Percepção** or
 * **Investigação** with the character's own bonus, then rolls (in the app, or types the face of the real d20, as
 * the campaign's dice setting allows) and reads the answer: "Você achou uma armadilha: Fosso escondido." or
 * "Você não encontrou nada." The same words for a roll that fell short and for no trap at all: the
 * helper line says only that "algumas armadilhas só se acham com Investigação", so the player learns
 * nothing about a particular trap. The three steps (Como, Rolar, Resultado) are drawn above; the roll
 * buttons are the combat's `RollPicker`. A Perception search with a typed die in dim light needs a second die
 * (`SEARCH_NEEDS_TWO_DICE`): the sheet asks for it with the reason, and the lower of the two counts at
 * the dim squares. In a combat it is the Search action and says so. Focus: the first radio; the result's
 * "Fechar".
 */
@Component({
  selector: 'app-trap-search-sheet',
  imports: [MatButtonModule, MatIconModule, RollPicker, SheetFrame],
  templateUrl: './trap-search-sheet.html',
  styleUrl: './trap-search-sheet.scss',
})
export class TrapSearchSheet {
  private readonly api = inject(TrapsClient);
  private readonly sheet = injectSheet<TrapSearchData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;

  protected readonly steps = SEARCH_STEPS;
  protected readonly options = skillOptions(this.data.skills);
  protected readonly skill = signal<SearchSkill>('perception');
  protected readonly chosen = computed(() => this.options.find((o) => o.skill === this.skill()) ?? this.options[0]);
  protected readonly typing = signal(false);
  /** The server asked for a second die: the first face typed waits here. */
  protected readonly firstFace = signal<number | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly result = signal<{ res: SearchForTrapsResponse; names: readonly string[]; skill: SearchSkill } | null>(null);
  protected readonly step = computed(() => searchStep(this.result() !== null, this.typing()));
  protected readonly short = mediaQuery('(max-height: 600px)');

  protected readonly canApp = this.data.diceMode !== DiceMode.PHYSICAL;
  protected readonly canType = this.data.diceMode !== DiceMode.APP;
  protected readonly preferApp = effectivePreference(this.data.diceMode, this.data.preference) === DicePreference.APP;

  protected readonly message = computed(() => resultMessage(this.result()?.names ?? []));
  protected readonly rolls = computed(() => {
    const r = this.result();
    return r ? [r.res.roll, r.res.secondRoll].filter((x) => x !== undefined).map((x) => rollText(x)) : [];
  });
  protected readonly total = computed(() => {
    const r = this.result()?.res;
    // With disadvantage the lower of the two counts: the server's roll is the one that counted first.
    return r?.roll?.total ?? 0;
  });
  protected readonly skillWord = computed(() => skillName(this.result()?.skill ?? this.skill()));
  protected readonly label = computed(() =>
    this.firstFace() !== null
      ? 'Digite o segundo dado'
      : `Role 1d20 para ${skillName(this.skill())}${this.data.skills ? ` (${this.chosen().title.replace(/^\S+\s/, '')})` : ''}`,
  );
  protected readonly hint = computed(() =>
    this.firstFace() !== null
      ? 'Há penumbra por perto: a procura tem desvantagem. Role de novo e digite o segundo dado (1 a 20); vale o menor onde está escuro.'
      : 'Role o seu dado e digite o número que saiu (1 a 20).',
  );

  private readonly key = newKey();
  private readonly frame = viewChild(SheetFrame);
  private readonly done = viewChild('done', { read: ElementRef<HTMLButtonElement> });
  protected readonly title = 'Procurar armadilhas';

  constructor() {
    effect(() => this.done()?.nativeElement.focus());
    effect(() => {
      if (this.error()) {
        this.frame()?.scrollToTop();
      }
    });
  }

  protected pick(skill: SearchSkill): void {
    this.skill.set(skill);
    this.firstFace.set(null);
  }

  protected async rollWith(die: SearchDie): Promise<void> {
    if (this.busy()) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      const first = this.firstFace();
      const sent: SearchDie = 'face' in die && first !== null ? { face: first, face2: die.face } : die;
      const res = await this.api.search(this.data.campaignId, this.skill(), sent, this.key);
      // Read the map again: what was found now shows on it, and its name is the map's to give.
      await this.data.state.refresh();
      const names = this.data.state
        .points()
        .filter((p) => p.kind === MapPointKind.TRAP && res.foundPointIds.includes(p.id))
        .map((p) => p.name);
      this.result.set({ res, names, skill: this.skill() });
      this.typing.set(false);
    } catch (err) {
      if (needsTwoDice(err) && 'face' in die && this.firstFace() === null) {
        this.firstFace.set(die.face);
        this.error.set(trapErrorMessage(err));
      } else {
        this.error.set(trapErrorMessage(err, 'procurar armadilhas'));
      }
    } finally {
      this.busy.set(false);
    }
  }

  protected cancel(): void {
    this.sheet.close(this.result() !== null);
  }
}

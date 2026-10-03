import { Component, ElementRef, computed, input, output, viewChild, afterNextRender } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import type { Combatant, Encounter } from '../../../../../gen/meurpg/play/v1/combat_pb';
import { combatantInitial, isPlayer } from '../../../../core/combat/combat-view';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import type { VitalsVm } from '../../live-session.types';
import type { CombatantInfo } from '../combat-info';

/** One player of "O grupo agora": the numbers as the master sees them. */
interface GroupRow {
  readonly c: Combatant;
  readonly hp: { current: number; max: number } | null;
  readonly slots: string;
  readonly down: string;
}

/**
 * "Combate encerrado" (E6-16): what the combat came to. The master reads the
 * rounds, who fell and how the party stands now (the live numbers, which the
 * combat never touches: they stay as they are, and "Ajustar" corrects them);
 * a player reads the same, without hit points. "Voltar à sessão" leaves the
 * summary for the session page; it is the page's one filled button.
 */
@Component({
  selector: 'app-combat-summary',
  imports: [CombatantToken, MatButtonModule, MatIconModule],
  templateUrl: './combat-summary.html',
  styleUrl: './combat-summary.scss',
})
export class CombatSummary {
  readonly encounter = input.required<Encounter>();
  readonly isMaster = input(false);
  readonly sessionNumber = input(0);
  readonly info = input<ReadonlyMap<string, CombatantInfo>>(new Map());
  /** The party's live numbers (the master's `vitals`). */
  readonly vitals = input<readonly VitalsVm[]>([]);

  readonly leave = output<void>();

  private readonly title = viewChild.required<ElementRef<HTMLElement>>('title');

  protected readonly players = computed(() => this.encounter().combatants.filter(isPlayer));
  protected readonly npcs = computed(() => this.encounter().combatants.filter((c) => !isPlayer(c)));
  protected readonly defeated = computed(() => this.npcs().filter((c) => c.defeated));
  protected readonly standing = computed(
    () => this.players().filter((c) => (c.hitPointsCurrent ?? 1) > 0).length,
  );
  protected readonly group = computed<GroupRow[]>(() =>
    this.players().map((c) => {
      const v = this.vitals().find((x) => x.characterId === c.characterId);
      const slots = (v?.spellSlots ?? [])
        .map((s) => `${s.level}º círculo: ${s.total - s.used} livres de ${s.total}`)
        .join(' · ');
      const down =
        v && v.hitPointsCurrent === 0
          ? `Caída: ${c.deathSuccesses} ${c.deathSuccesses === 1 ? 'sucesso' : 'sucessos'}, ${c.deathFailures} ${c.deathFailures === 1 ? 'falha' : 'falhas'}. Precisa de cura.`
          : '';
      return {
        c,
        hp: v ? { current: v.hitPointsCurrent, max: v.hitPointsMax } : null,
        slots,
        down,
      };
    }),
  );
  protected readonly downNotice = computed(() => this.group().filter((g) => g.down));
  protected readonly rounds = computed(() => Math.max(1, this.encounter().round));

  constructor() {
    // The summary replaces the combat, so the screen reader starts at it.
    afterNextRender(() => this.title().nativeElement.focus());
  }

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected sub(c: Combatant): string {
    const info = this.info().get(c.characterId);
    return isPlayer(c) ? (info?.classSummary ?? '') : (info?.kindLabel ?? 'NPC');
  }

  protected percent(g: GroupRow): number {
    return g.hp && g.hp.max > 0 ? Math.round((g.hp.current / g.hp.max) * 100) : 0;
  }
}

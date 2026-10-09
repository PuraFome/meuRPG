import { NgTemplateOutlet } from '@angular/common';
import { Component, ElementRef, computed, effect, input, output, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

import {
  type Combatant,
  CombatantState,
  type Encounter,
} from '../../../../../gen/meurpg/play/v1/combat_pb';
import { joinDots, tight } from '../../../../core/format/text';
import { metersFixed, squaresFree } from '../../../../core/units';
import { article } from '../../../../core/combat/combat-log';
import {
  combatantInitial,
  isDead,
  isDown,
  isPlayer,
  ownCombatant,
  playerWord,
  roundLabel,
  stateWord,
  turnBanner,
} from '../../../../core/combat/combat-view';
import { isCreature } from '../../../../core/combat/creature-names';
import { conditionTags } from '../../../../core/combat/conditions';
import {
  leftSentence,
  listNames,
  missingLine,
  passNote,
  playsBefore,
} from '../../../../core/combat/joint-turn';
import { mediaQuery } from '../../../../shared/map-view/media-query';
import { WildBand } from '../../../../shared/wild-shape/wild-band';
import type { FallNote } from '../../../../core/traps/trap-log';
import { CombatantToken } from '../../../../shared/combatant-token/combatant-token';
import { EndPart } from '../joint-turn/end-part';
import { JointOthers } from '../joint-turn/joint-others';
import { JointPill } from '../joint-turn/joint-pill';
import { CombatantTags } from '../combatant-tags/combatant-tags';
import { RageStatus } from '../rage-end/rage-status';
import { EndTurn } from './end-turn';
import { OrderStrip } from './order-strip';
import { ConcentrationLine, TurnReaction } from './turn-extras';

/**
 * What a player sees at the top of a running combat (E6-05, E6-06): whose
 * turn it is, who is next, and the order as a strip of chips. The banner is
 * a live region, so a turn change is heard once. When the turn is a hidden
 * combatant's it says "Vez do mestre", with no name and no highlighted chip.
 * On the player's own turn ("Sua vez") the banner is the hero, with the
 * movement left and, from 1024px, "Encerrar turno" (`EndTurn`). A character at
 * 0 hit points gets the same hero in the danger frame instead ("Brisa está
 * caída", E6-13; the death saves are the page's `DeathSaves`). Off turn it
 * offers the opportunity attack under "Sua reação" (E6-28). The groups
 * of actions (`ActionGroups`) come right after it, with "Mover" in the
 * Movimento group. Focus goes to the hero when the turn arrives, so the next
 * Tab reaches what the player can do.
 */
@Component({
  selector: 'app-turn-panel',
  imports: [
    CombatantTags,
    CombatantToken,
    ConcentrationLine,
    EndPart,
    EndTurn,
    JointOthers,
    JointPill,
    MatIconModule,
    NgTemplateOutlet,
    OrderStrip,
    RageStatus,
    TurnReaction,
    WildBand,
  ],
  templateUrl: './turn-panel.html',
  styleUrl: './turn-panel.scss',
})
export class TurnPanel {
  readonly encounter = input.required<Encounter>();
  readonly busy = input(false);
  /** Extra Attack: how many attacks remain after the first spent the action
   * (`TurnEconomy`), and how many the Attack action makes. */
  readonly attacksLeft = input(0);
  readonly attacksPerAction = input(1);
  /** The player's maximum hit points, for "com 0 de 24 pontos de vida". */
  readonly hitPointsMax = input<number | null>(null);
  /** The melee attacks an opportunity attack can use (off turn, reaction free). */
  readonly opportunities = input<readonly { key: string; name: string }[]>([]);
  /** An opportunity attack waits for an answer: the title ("Esperando a reação do mestre") and the line under it (E9-13). */
  readonly waiting = input<{ readonly title: string; readonly detail: string } | null>(null);
  /** What the last move said when it stopped short ("Você parou antes: algo bloqueou o caminho."). */
  readonly moveNote = input('');
  /** What a trap did to this player's character this round (E9-08 E), or `null`. */
  readonly trapNote = input<FallNote | null>(null);
  /** The name of the spell the player is concentrating on, or `''`. */
  readonly concentration = input('');
  /** The combat is played without a map (RN-25): the movement tile has no squares to count. */
  readonly theatre = input(false);
  /** The page draws the order strip itself, under the actions, on the player's own turn on a phone. */
  readonly orderBelow = input(false);

  readonly endTurn = output<void>();
  /** "Ataque de oportunidade": the key of the melee attack. */
  readonly opportunity = output<string>();
  readonly endConcentration = output<void>();
  /** "Encerrar fúria": the id of the raging character. */
  readonly endRage = output<string>();

  private readonly hero = viewChild<ElementRef<HTMLElement>>('hero');

  /** From 1024px the player's own turn is a compact banner (E6-14); from
   * 1280px the order is a column on the left, so the strip goes. */
  private readonly desktop = mediaQuery('(min-width: 1024px)');
  protected readonly wide = mediaQuery('(min-width: 1280px)');
  protected readonly compact = computed(() => this.desktop() && this.banner().mine && !this.down());
  protected readonly banner = computed(() => turnBanner(this.encounter()));
  /** The members of a joint turn that is all creatures (the wolves): the header draws each one's token. */
  protected readonly groupTokens = computed<readonly Combatant[]>(() => {
    const members = this.banner().joint?.members ?? [];
    return members.length > 1 && members.every(isCreature) ? members : [];
  });
  protected readonly round = computed(() => roundLabel(this.encounter().round));
  protected readonly own = computed(() => ownCombatant(this.encounter()));
  /** The beast the druid is in, with its own reserve of hit points (only the druid's player and the master get the numbers). */
  protected readonly form = computed(() => {
    const c = this.own();
    return c?.wildShapeBeastKey
      ? {
          name: c.wildShapeBeastNamePt,
          current: c.wildShapeHitPointsCurrent,
          max: c.wildShapeHitPointsMax,
        }
      : null;
  });
  /** "Sem magias · 12,0 m": the beast's armor class is the one on the vitals card, once on the page. */
  protected readonly formDetail = computed(() => {
    const c = this.own();
    return c?.wildShapeBeastKey
      ? joinDots(['Sem magias', tight(metersFixed(c.speedDft / 10))])
      : '';
  });
  /** The player looks through their familiar's eyes: the character is blind and does not attack (the master resolves it, MR-036). */
  protected readonly blind = computed(() => !!this.own()?.familiarSightCreatureId);
  /** The player's character is at 0 hit points (any turn). */
  protected readonly dead = computed(() => {
    const me = this.own();
    return !!me && isDead(me);
  });
  protected readonly isDown = computed(() => {
    const own = this.own();
    return !!own && isDown(own);
  });
  /** It is their turn and they are down: the hero is the danger one. */
  protected readonly down = computed(() => this.isDown() && this.banner().mine);
  protected readonly downWord = computed(() =>
    article(this.own()?.label ?? '') === 'a' ? 'caída' : 'caído',
  );
  /** The line of a fallen character off turn: what is owed, or that the master decides. */
  protected readonly offTurnDown = computed(() => {
    const own = this.own();
    if (!own) {
      return '';
    }
    if (own.state === CombatantState.STABLE) {
      return `${own.label} está estável.`;
    }
    return own.deathFailures >= 3
      ? `${own.label} está ${this.downWord()}, com três falhas: o mestre decide.`
      : `${own.label} está ${this.downWord()}. Na sua vez, role o teste contra a morte.`;
  });
  protected readonly downTitle = computed(() =>
    this.down() ? `${this.own()?.label} está ${this.downWord()}` : '',
  );
  protected readonly downText = computed(() => {
    const own = this.own();
    const max = this.hitPointsMax();
    const pv = max === null ? '0 pontos de vida' : `0 de ${max} pontos de vida`;
    if (own?.state === CombatantState.STABLE) {
      return `É a sua vez, mas com ${pv} você está estável e não age.`;
    }
    return own?.deathSaveDue
      ? `É a sua vez, mas com ${pv} você não age. Role o teste contra a morte.`
      : `É a sua vez, mas com ${pv} você não age.`;
  });
  /** The tiles of the hero: with Extra Attack the Ação tile counts the attacks left. */
  protected readonly tiles = computed(() => {
    const c = this.own();
    if (!c) {
      return [];
    }
    const left = this.attacksLeft();
    const partial = c.actionUsed && left > 0 && this.attacksPerAction() > 1;
    return [
      {
        name: 'Ação',
        used: c.actionUsed && !partial,
        word: partial
          ? `${left} ${left === 1 ? 'ataque restante' : 'ataques restantes'}`
          : c.actionUsed
            ? 'Usada'
            : 'Disponível',
      },
      {
        name: 'Ação bônus',
        used: c.bonusActionUsed,
        word: c.bonusActionUsed ? 'Usada' : 'Disponível',
      },
      { name: 'Reação', used: c.reactionUsed, word: c.reactionUsed ? 'Usada' : 'Disponível' },
    ];
  });
  protected readonly strip = computed(() => this.encounter().combatants);
  protected readonly movement = computed(() => {
    const own = this.own();
    return own
      ? {
          // Tenths of a foot, metres with one decimal: "6,9 m de 9,0 m".
          left: metersFixed(own.movementLeftDft / 10),
          total: metersFixed(own.speedDft / 10),
          free: squaresFree(own.movementLeftFt),
          used: own.movementUsedDft > 0 ? metersFixed(own.movementUsedDft / 10) : '',
          none: own.movementLeftDft <= 0,
          percent: Math.max(
            0,
            Math.min(100, (own.movementLeftDft / Math.max(1, own.speedDft)) * 100),
          ),
        }
      : null;
  });
  /** The pill of a joint turn: "Turno conjunto com Brisa" for a member, "Turno
   * conjunto" for the other players. A group of NPCs alone has none (RN-20). */
  protected readonly pill = computed(() => {
    const joint = this.banner().joint;
    if (!joint || joint.npcOnly) {
      return '';
    }
    const others = joint.members.filter((m) => !m.mine).map((m) => m.label);
    return joint.members.some((m) => m.mine) && others.length > 0
      ? `Turno conjunto com ${listNames(others)}`
      : 'Turno conjunto';
  });
  protected readonly afterPrefix = computed(() =>
    this.banner().joint ? 'Depois de vocês' : 'Depois de você',
  );
  /** "O turno passa quando você e a Brisa encerrarem." */
  protected readonly footerNote = computed(() => {
    const joint = this.banner().joint;
    return passNote(
      (joint?.acting ?? []).filter((m) => !m.mine).map((m) => m.label),
      !!joint?.waitsForMaster,
    );
  });
  /** What the own part still has, for the question before ending it. */
  protected readonly partLeft = computed(() => {
    const c = this.own();
    return c ? leftSentence(c) : '';
  });
  /** "Falta a Brisa. O turno passa quando ela encerrar a parte dela." */
  protected readonly missing = computed(() => {
    const joint = this.banner().joint;
    return joint
      ? missingLine(
          joint.acting.filter((m) => !m.mine).map((m) => m.label),
          joint.waitsForMaster,
        )
      : '';
  });
  /** A player whose group is not on turn, with a joint turn of players going on. */
  protected readonly outsiderJoint = computed(() => {
    const joint = this.banner().joint;
    return !!joint && !joint.npcOnly && !joint.members.some((m) => m.mine);
  });
  /** "do Capitão Goblin": who plays right before the player's own turn. */
  protected readonly before = computed(() => playsBefore(this.encounter()));
  protected readonly nextLine = computed(() => {
    const banner = this.banner();
    if (banner.mine || banner.masterTurn || !banner.next) {
      return null;
    }
    return banner.nextIsMine
      ? { prefix: 'Você é o próximo: depois dele, ', name: banner.next.label }
      : null;
  });

  constructor() {
    // Focus moves to the hero when the turn arrives, once, so the next Tab
    // reaches "Mover".
    let wasMine = false;
    effect(() => {
      const mine = this.banner().mine;
      if (mine && !wasMine) {
        queueMicrotask(() => this.hero()?.nativeElement.focus());
      }
      wasMine = mine;
    });
  }

  protected initial(c: Combatant): string {
    return combatantInitial(c.label);
  }

  protected conditions(c: Combatant): string[] {
    return conditionTags(c);
  }

  protected npc(c: Combatant): boolean {
    return !isPlayer(c) && !isCreature(c);
  }

  protected creature(c: Combatant): boolean {
    return isCreature(c);
  }

  /** The chip's second line: an NPC's state word, "Jogador" for another
   * player (their name is not sent to the other players). */
  protected word(c: Combatant): string {
    return isPlayer(c) ? playerWord(c) : stateWord(c.state);
  }

  protected current(c: Combatant): boolean {
    return c.id === this.encounter().currentCombatantId;
  }
}

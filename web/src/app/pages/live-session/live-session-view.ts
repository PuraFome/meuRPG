import { computed } from '@angular/core';

import { CombatantKind, EncounterStatus } from '../../../gen/meurpg/play/v1/combat_pb';
import { acts } from '../../core/combat/joint-turn';
import { mineTabs } from '../../core/combat/mine';
import { isTheatre } from '../../core/combat/theatre';
import { returnPlace } from '../../core/revivify/revivify-flow';
import { OwnEffects } from './effects/own-effects';
import type { LiveSessionParts } from './live-session-parts';
import { highlightsSubOf } from './live-session-pure';
import type { LiveSessionState } from './live-session-state';

/** What the session page derives from its state and the combat, the map and the notes it holds. */
export class LiveSessionView {
  /** The current map's ID when it has the fog on and the session is live; the master reads it too (his own map is drawn by the same component). */
  readonly fogMapId = computed(() => {
    const map = this.parts.mapState.map();
    return map?.fogEnabled && this.state.phase() === 'live' ? map.id : null;
  });
  readonly fogMaster = computed(
    () => this.state.isMaster() && !!this.parts.mapState.map()?.fogEnabled,
  );
  readonly fogPlayer = computed(
    () => !this.state.isMaster() && !!this.parts.mapState.map()?.fogEnabled,
  );
  readonly ownToken = computed(
    () => this.parts.mapState.tokens().find((t) => t.mine && !t.creatureId) ?? null,
  );
  /** The running combat has no map (RN-25): no fog, no traps, nothing that depends on place. */
  readonly inTheatre = computed(() => isTheatre(this.parts.combat.shown()));
  /** Where it returns in the order and when it next acts, from the combat on screen. */
  readonly revivedPlace = computed(() =>
    returnPlace(this.parts.combat.encounter(), this.state.revivedId()),
  );
  /** A combat is running (not ended): the player's open puzzle page keeps a notice for it, and says when it is their turn. */
  readonly combatRunning = computed(
    () =>
      !this.state.isMaster() && this.parts.combat.encounter()?.status === EncounterStatus.ACTIVE,
  );
  readonly yourTurn = computed(() => {
    const e = this.parts.combat.encounter();
    return (
      !!e && e.status === EncounterStatus.ACTIVE && mineTabs(e).some((tab) => tab.state === 'turn')
    );
  });
  /** The master's puzzle panel is on the board when the campaign has puzzles (or when they could not be read). */
  readonly puzzlesShown = computed(
    () => this.parts.puzzles.runs().length > 0 || this.parts.puzzles.status() === 'error',
  );
  readonly freshCount = computed(() => this.parts.notes.fresh().length);
  /** The player's own combatant in the combat, which a character that died keeps (it has no vitals). */
  private readonly ownCombatant = computed(() =>
    this.parts.combat
      .encounter()
      ?.combatants.find((c) => c.mine && c.kind === CombatantKind.PLAYER),
  );
  /** The player's character: from the vitals, or, for one that died, from its combatant. */
  readonly ownCharacterName = computed(
    () => this.state.vitals().at(0)?.name || this.ownCombatant()?.label || '',
  );
  /** Whose numbers the "O combate acabou" card marks as the reader's: a dead character's too. */
  readonly highlightsCharacterId = computed(
    () => this.state.ownCharacterId() || this.ownCombatant()?.characterId || '',
  );
  /** The players' "O combate acabou" card: the combat that ended, until they close it. */
  readonly highlightsFor = computed(() => {
    const e = this.parts.combat.encounter();
    return !this.state.isMaster() &&
      e?.status === EncounterStatus.ENDED &&
      e.id !== this.state.highlightsClosed()
      ? e
      : null;
  });
  readonly highlightsSub = computed(() => highlightsSubOf(this.highlightsFor()));
  /** The combat view (the order, the log, the summary) is on screen: always for the master; for a player, not once the combat ended and
   * the "O combate acabou" card carries what it came to, so the scene, the map and the rest of the page stay under that one card. */
  readonly combatOnScreen = computed(() => {
    const e = this.parts.combat.shown();
    return e !== null && (this.state.isMaster() || e.status !== EncounterStatus.ENDED);
  });
  /** The Escudo Arcano's bonus on the player's own combatant while a combat is on; `null` outside one, so a combat that
   * ends is not read as the shield ending. */
  readonly ownArmorBonus = computed(() => {
    const e = this.parts.combat.encounter();
    if (this.state.isMaster() || e?.status !== EncounterStatus.ACTIVE) {
      return null;
    }
    return e.combatants.find((c) => c.mine)?.armorClassBonus ?? null;
  });
  /** Whether it is the turn of the player's own combatant (it acts now): the death saves are rolled then. */
  readonly ownTurn = computed(() => {
    const e = this.parts.combat.encounter();
    const own = e?.status === EncounterStatus.ACTIVE ? e.combatants.find((c) => c.mine) : undefined;
    return !!e && !!own && acts(e, own);
  });
  /** The death saves of the player's own combatant while a combat is on; `null` outside one. */
  readonly ownDeathSaves = computed(() => {
    const e = this.parts.combat.encounter();
    if (this.state.isMaster() || e?.status !== EncounterStatus.ACTIVE) {
      return null;
    }
    const own = e.combatants.find((c) => c.mine);
    return own ? { successes: own.deathSuccesses, failures: own.deathFailures } : null;
  });
  /** "Seus efeitos" and the labels of the conditions they hold, for the player's own sheet (RN-22). */
  readonly ownEffects: OwnEffects;
  /** The player may search the map for traps: a character of theirs, on a map with a grid. */
  readonly searchable = computed(
    () =>
      !this.state.isMaster() &&
      this.state.ownVitals() !== null &&
      (this.parts.mapState.map()?.gridColumns ?? 0) > 0,
  );

  constructor(
    private readonly state: LiveSessionState,
    private readonly parts: LiveSessionParts,
  ) {
    this.ownEffects = new OwnEffects({
      campaignId: state.campaignId,
      isMaster: state.isMaster,
      ownCharacterIds: computed(() => state.vitals().map((v) => v.characterId)),
      encounter: parts.combat.encounter,
      tick: computed(() => state.castsTick() + (state.ownVitals()?.revision ?? 0)),
    });
  }
}

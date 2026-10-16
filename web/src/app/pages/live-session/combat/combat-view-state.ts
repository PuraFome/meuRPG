import { computed, signal, Directive } from '@angular/core';
import { EncounterStatus } from '../../../../gen/meurpg/play/v1/combat_pb';
import { isTheatre } from '../../../core/combat/theatre';
import { CreatureOptionsState } from '../../../core/combat/creature-options-state';
import { CHARACTER_TAB, type MineTab, mineTabs } from '../../../core/combat/mine';
import { offersToAnswer } from '../../../core/combat/opportunity';
import { LayersState } from '../../../core/maps/layers-state';
import { CombatLogState } from '../../../core/combat/combat-log-state';
import { TurnOptionsState } from '../../../core/combat/turn-options-state';
import { currentCombatant, isDown, ownCombatant } from '../../../core/combat/combat-view';
import { acts, jointTurn, masterActor } from '../../../core/combat/joint-turn';
import { MasterAutoPass } from '../../../core/combat/auto-pass';
import { CombatCalls } from '../../../core/combat/combat-calls';
import { CombatReads } from './combat-reads';
import { CombatViewInputsDirective } from './combat-view-inputs';

/** The core of what the screen shows: the combat, who the player is in it, whose turn it is and the services' state
 * the parts read (`CombatCalls`, `CombatReads`, `MasterAutoPass`). */
@Directive()
export abstract class CombatViewStateDirective extends CombatViewInputsDirective {
  /** The character's own vitals row: the hit points that wait while it is a beast, and the uses of its resources. */
  protected readonly ownVitals = computed(() => {
    const own = this.own();
    return own ? (this.vitals().find((v) => v.characterId === own.characterId) ?? null) : null;
  });

  /** The calls that answer with the combat: one at a time, the answer applied, a refusal said (`CombatCalls`). */
  protected readonly calls = new CombatCalls({
    api: this.api,
    campaignId: () => this.campaignId(),
    encounter: () => this.encounter(),
    state: () => this.state(),
  });
  protected readonly busy = this.calls.busy;
  protected readonly error = this.calls.error;

  /** What the screen reads from the services around the combat (`CombatReads`). */
  protected readonly reads = new CombatReads({
    campaignId: () => this.campaignId(),
    isMaster: () => this.isMaster(),
    encounter: () => this.encounter(),
    own: () => this.own(),
    options: () => this.options(),
    tableRules: this.tableRules,
    catalog: this.catalog,
    roster: this.roster,
    creatures: this.creaturesApi,
  });
  protected readonly rosterInfo = this.reads.rosterInfo;
  protected readonly spellDetails = this.reads.spellDetails;
  protected readonly revealSpellNames = this.reads.revealSpellNames;
  protected readonly beastBlock = this.reads.beastBlock;
  protected readonly familiar = this.reads.familiar;
  protected readonly deathsHidden = this.reads.deathsHidden;

  /** The master's screen passes by itself the reactions a player leaves unanswered (`MasterAutoPass`). */
  protected readonly autoPass = new MasterAutoPass({
    api: this.api,
    campaignId: () => this.campaignId(),
    encounter: () => this.encounter(),
    isMaster: () => this.isMaster(),
    state: () => this.state(),
  });
  protected readonly autoLeft = this.autoPass.windowsLeft;
  protected readonly offerLeft = this.autoPass.offersLeft;
  /** What the combatant in the spotlight can do: the player's own on their
   * turn, the master's current one (`GetTurnOptions`). */
  protected readonly turn = new TurnOptionsState();
  protected readonly log = new CombatLogState();
  protected readonly encounter = computed(() => this.state().shown());
  protected readonly image = computed(() => {
    const e = this.encounter();
    const map = this.mapState().map();
    return e && map && map.id === e.mapId && map.image
      ? { url: map.image.url, width: map.image.width, height: map.image.height }
      : null;
  });
  protected readonly mapName = computed(() => this.mapState().map()?.name ?? '');
  /** The combat is played without a map (RN-25): a list instead of a map, movement by number, the master's offers (E10-04). */
  protected readonly theatre = computed(() => isTheatre(this.encounter()));
  protected readonly lookingThroughFamiliar = computed(() => !!this.own()?.familiarSightCreatureId);
  protected readonly own = computed(() => {
    const e = this.encounter();
    return e ? ownCombatant(e) : null;
  });
  protected readonly myTurn = computed(() => {
    const e = this.encounter();
    const own = this.own();
    return !!e && !!own && acts(e, own);
  });
  /** The tabs of the turn bar: the character, then each group of creatures (a casting's, in a joint turn, or one alone). */
  protected readonly tabs = computed<readonly MineTab[]>(() => {
    const e = this.encounter();
    return !this.isMaster() && e && e.status === EncounterStatus.ACTIVE ? mineTabs(e) : [];
  });
  /** The tabs show only when the player plays more than one combatant. */
  protected readonly showTabs = computed(() => this.tabs().length > 1);
  /** The tab the page shows; it follows the turn to where the player acts, and a tap changes it. */
  protected readonly tabId = signal(CHARACTER_TAB);
  protected readonly tab = computed<MineTab | null>(
    () => this.tabs().find((t) => t.id === this.tabId()) ?? this.tabs()[0] ?? null,
  );
  /** The page of a creature's turn is open (not the character's). */
  protected readonly creatureTab = computed(() => {
    const t = this.tab();
    return t?.creature ? t : null;
  });
  /** `GetTurnOptions` of each creature the player plays: the attacks with their numbers, the standard actions. */
  protected readonly creatureOptions = new CreatureOptionsState();
  /** The question to end a group's part is open: the tabs are inert until it is answered. */
  protected readonly partAsking = signal(false);
  /** The joint turn that is running (a group of two or more), or `null`. */
  protected readonly joint = computed(() => {
    const e = this.encounter();
    return e ? jointTurn(e) : null;
  });
  /** In a joint turn, who else must end their part, for the player's footer. */
  protected readonly jointOthers = computed(() => {
    const joint = this.joint();
    if (!joint || !this.myTurn()) {
      return null;
    }
    return [
      ...joint.acting.filter((m) => !m.mine).map((m) => m.label),
      ...(joint.waitsForMaster ? ['o mestre'] : []),
    ];
  });
  /** The player's own slots, for the rows above the spell list. */
  protected readonly ownSlots = computed(() => {
    const own = this.own();
    const v = own ? this.vitals().find((x) => x.characterId === own.characterId) : undefined;
    return { usage: v?.spellSlots ?? [], pact: v?.pactSlots ?? null };
  });
  /** The map's painted layers (walls, difficult terrain, cover), read again when the map's `layers_revision` changes. */
  protected readonly layersState = new LayersState(
    async (mapId) => this.maps.layers(this.campaignId(), mapId),
    () => !this.isMaster(),
  );
  /** On a fog map the player's layers come with the vision (filtered to what they see); otherwise they are the map's. */
  protected readonly fogVision = computed(() =>
    this.isMaster() ? null : (this.fog()?.vision() ?? null),
  );
  protected readonly layers = computed(() =>
    this.fogVision()
      ? (this.fog()?.layers() ?? this.layersState.layers())
      : this.layersState.layers(),
  );
  /** The offers the caller answers, and the ones on the player's own mover. */
  protected readonly toAnswer = computed(() => {
    const e = this.encounter();
    return e ? offersToAnswer(e) : [];
  });
  /** The one whose options the screen asks for: the player's own character
   * on their turn, and for the master whoever is on turn (in a joint turn that holds a player, the NPC that still acts). */
  protected readonly subject = computed(() => {
    const e = this.encounter();
    if (!e || e.status !== EncounterStatus.ACTIVE) {
      return null;
    }
    // A player's options are asked off turn too: the opportunity attack
    // (`attack_targets` is filled, `as_reaction`) needs them.
    return this.isMaster() ? (masterActor(e) ?? currentCombatant(e)) : this.own();
  });
  protected readonly options = computed(() => this.turn.data());
  /** The player's maximum hit points, for "com 0 de 24 pontos de vida". */
  protected readonly ownMaxHp = computed(
    () =>
      this.vitals().find((v) => v.characterId === this.own()?.characterId)?.hitPointsMax ?? null,
  );
  protected readonly ownDown = computed(() => {
    const own = this.own();
    return !!own && isDown(own);
  });
  protected readonly setup = computed(() => this.encounter()?.status === EncounterStatus.SETUP);
  protected readonly active = computed(() => this.encounter()?.status === EncounterStatus.ACTIVE);
  protected readonly ended = computed(() => this.encounter()?.status === EncounterStatus.ENDED);
  /** Changes only when the turn passes (or the combat ends): a string, so the effects that read it ignore the combat's other updates. */
  protected readonly turnKey = computed(() => {
    const e = this.encounter();
    return e && e.status === EncounterStatus.ACTIVE
      ? `${e.id}:${e.round}:${e.currentCombatantId}`
      : '';
  });
}

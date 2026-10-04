import { Component, computed, effect, inject, input, output, signal, untracked } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { Code, ConnectError } from '@connectrpc/connect';

import type { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import {
  CombatLogKind,
  CombatantState,
  DeathSaveOutcome,
  type Encounter,
  EncounterStatus,
  type PendingDamage,
  PendingDamageStatus,
  type ReactionPrompt,
} from '../../../../gen/meurpg/play/v1/combat_pb';
import { ActionEconomy, type Attack, AttackKind } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { type AttackDie, CombatClient, newKey } from '../../../core/combat/combat-client';
import { combatErrorMessage } from '../../../core/combat/combat-errors';
import { type Square, canReach, reachSquares } from '../../../core/combat/combat-grid';
import { openDamages, pendingNote } from '../../../core/combat/attack-flow';
import { CombatLogState } from '../../../core/combat/combat-log-state';
import type { CombatState } from '../../../core/combat/combat-state';
import { TurnOptionsState } from '../../../core/combat/turn-options-state';
import { saveAnnouncement } from '../../../core/combat/death-saves';
import { currentCombatant, isDown, isPlayer, ownCombatant, npcKindLabel } from '../../../core/combat/combat-view';
import type { MapState } from '../../../core/maps/map-state';
import { MoveSaves } from '../../../core/maps/move-saves';
import { RosterClient } from '../../../core/maps/roster-client';
import type { TokenDrop } from '../../../shared/combat-map/combat-map';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import type { PartyMemberInfoVm, VitalsVm } from '../live-session.types';
import { ActionGroups } from './action-groups/action-groups';
import { AdjustNpc, type AdjustNpcData } from './adjust-npc/adjust-npc';
import { AttackSheet, type AttackSheetData } from './attack-sheet/attack-sheet';
import { CastSheet, type CastSheetData } from './cast-sheet/cast-sheet';
import { ConditionsDialog, type ConditionsData } from './conditions-dialog/conditions-dialog';
import { DeathQuestion } from './death-question/death-question';
import { DeathSaves } from './death-saves/death-saves';
import { FeatureSheet, type FeatureSheetData } from './feature-sheet/feature-sheet';
import { ShieldSheet, type ShieldSheetData } from './shield-sheet/shield-sheet';
import { CombatLogPanel } from './combat-log/combat-log-panel';
import type { CombatantInfo } from './combat-info';
import { CombatBar } from './combat-bar/combat-bar';
import { CombatMapCard } from './combat-map-card/combat-map-card';
import { CombatSummary } from './combat-summary/combat-summary';
import { InitiativeSetup } from './initiative-setup/initiative-setup';
import { InitiativeSide } from './initiative-side/initiative-side';
import { MovePage } from './move-page/move-page';
import { NpcCard } from './npc-card/npc-card';
import { OrderList } from './order-list/order-list';
import { PlayerInitiative } from './player-initiative/player-initiative';
import { openSheet } from './sheet-host';
import { type StartCombatData, StartCombatDialog } from './start-combat/start-combat-dialog';
import { OrderColumn } from './turn-panel/order-column';
import { TurnBar } from './turn-panel/turn-bar';
import { TurnPanel } from './turn-panel/turn-panel';

/**
 * The combat on the session page (MR-013, E6-01 to E6-16): it picks the
 * screen by who is looking and where the combat is, and runs every call.
 *
 * - **Master:** SETUP is the initiative list with its side panel (E6-04);
 *   ACTIVE is the combat bar, the map and the order (E6-11, E6-12); ENDED is
 *   the summary (E6-16).
 * - **Player:** SETUP is "Role a iniciativa" (E6-03); ACTIVE is the turn
 *   banner with the order strip, the map and, projected in, the vitals
 *   (E6-05), or the "Mover" page (E6-10); ENDED is the summary.
 *
 * It never reads the combat by itself: the page does, after
 * `encounter_changed`. A call's answer is the combat as it is now, so it is
 * applied at once; a refusal that means "the screen is stale" reads it again.
 */
@Component({
  selector: 'app-combat-view',
  imports: [
    ActionGroups,
    DeathQuestion,
    DeathSaves,
    CombatBar,
    CombatLogPanel,
    CombatMapCard,
    CombatSummary,
    InitiativeSetup,
    InitiativeSide,
    MatButtonModule,
    MatIconModule,
    MovePage,
    NpcCard,
    OrderColumn,
    OrderList,
    PlayerInitiative,
    TurnBar,
    TurnPanel,
  ],
  templateUrl: './combat-view.html',
  styleUrl: './combat-view.scss',
})
export class CombatView {
  private readonly api = inject(CombatClient);
  private readonly roster = inject(RosterClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  protected readonly phone = mediaQuery(PHONE_QUERY);
  /** From 1024px the master has the combat bar (E6-11); below it the turn card does it all (E6-12). */
  protected readonly laptop = mediaQuery('(min-width: 1024px)');
  /** From 1280px the player's order is a column on the left (E6-14). */
  protected readonly wideLayout = mediaQuery('(min-width: 1280px)');
  /** One save in flight per combatant (see `MoveSaves`). */
  private readonly moves = new MoveSaves();

  readonly campaignId = input.required<string>();
  readonly isMaster = input(false);
  readonly state = input.required<CombatState>();
  readonly mapState = input.required<MapState>();
  readonly vitals = input<readonly VitalsVm[]>([]);
  readonly partyInfo = input<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());
  readonly sessionNumber = input(0);
  /** The players' "O combate acabou" card is above: the summary does not repeat its heading on screen. */
  readonly cardAbove = input(false);
  /** The player's own armor class from their sheet (without Escudo's +5), for the Escudo result. */
  readonly armorClass = input<number | null>(null);
  readonly diceMode = input.required<DiceMode>();
  readonly dicePreference = input.required<DicePreference>();

  /** "Dano/Cura" on a player's row: the page opens its adjust dialog. */
  readonly adjust = output<VitalsVm>();
  /** "Mudar" (how the player rolls). */
  readonly changeDice = output<void>();

  protected readonly Status = EncounterStatus;
  /** What the combatant in the spotlight can do: the player's own on their
   * turn, the master's current one (`GetTurnOptions`). */
  protected readonly turn = new TurnOptionsState();
  protected readonly log = new CombatLogState();
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  /** A refusal of the move on the "Mover" page (TOO_FAR, SQUARE_OCCUPIED). */
  protected readonly moveError = signal('');
  protected readonly rosterInfo = signal<ReadonlyMap<string, CombatantInfo>>(new Map());

  /** What the last feature said ("Surto de Ação: você tem outra ação"), and the
   * death save just rolled this turn; both are cleared when the turn changes. */
  protected readonly actionNote = signal('');
  protected readonly deathResult = signal('');
  /** The turn whose damage was just settled: the master's card stays for its note. */
  private readonly settledTurn = signal('');
  /** The characters at three failures the master put away with "Ainda não". */
  protected readonly deathLater = signal<ReadonlySet<string>>(new Set());
  private deathKey = newKey();
  private readonly shieldHandled = new Set<string>();

  protected readonly encounter = computed(() => this.state().shown());
  protected readonly image = computed(() => {
    const e = this.encounter();
    const map = this.mapState().map();
    return e && map && map.id === e.mapId && map.image
      ? { url: map.image.url, width: map.image.width, height: map.image.height }
      : null;
  });
  protected readonly mapName = computed(() => this.mapState().map()?.name ?? '');
  protected readonly info = computed<ReadonlyMap<string, CombatantInfo>>(() => {
    const merged = new Map(this.rosterInfo());
    // A player has no roster: their own combatant says what the table needs.
    for (const [id, p] of this.partyInfo()) {
      if (!merged.has(id)) {
        merged.set(id, { classSummary: p.classSummary, playerName: p.playerName, kindLabel: '', raceName: '' });
      }
    }
    return merged;
  });
  /** The characters whose "Dano/Cura" has a vitals row to adjust. */
  protected readonly adjustable = computed(() => new Set(this.vitals().map((v) => v.characterId)));
  protected readonly own = computed(() => {
    const e = this.encounter();
    return e ? ownCombatant(e) : null;
  });
  protected readonly myTurn = computed(() => {
    const e = this.encounter();
    return !!e && !!this.own() && e.currentCombatantId === this.own()?.id;
  });
  /** The squares a player's own token may be dragged to, on their turn. */
  protected readonly reach = computed(() => {
    const own = this.own();
    return this.myTurn() && own && own.placed
      ? { origin: { col: own.col, row: own.row }, squares: reachSquares(own.movementLeftFt) }
      : null;
  });
  /** The one whose options the screen asks for: the player's own character
   * on their turn, and for the master whoever is on turn. */
  protected readonly subject = computed(() => {
    const e = this.encounter();
    if (!e || e.status !== EncounterStatus.ACTIVE) {
      return null;
    }
    // A player's options are asked off turn too: the opportunity attack
    // (`attack_targets` is filled, `as_reaction`) needs them.
    return this.isMaster() ? currentCombatant(e) : this.own();
  });
  protected readonly options = computed(() => this.turn.data());
  protected readonly economy = computed(() => this.options()?.options?.economy);
  /** Extra Attack: the attacks of this Attack action that remain, and how many it makes. */
  protected readonly attacksLeft = computed(() => this.economy()?.attacksLeft ?? 0);
  protected readonly attacksPerAction = computed(() => this.economy()?.attacksPerAction ?? 1);
  /** The player's maximum hit points, for "com 0 de 24 pontos de vida". */
  protected readonly ownMaxHp = computed(
    () => this.vitals().find((v) => v.characterId === this.own()?.characterId)?.hitPointsMax ?? null,
  );
  protected readonly ownDown = computed(() => {
    const own = this.own();
    return !!own && isDown(own);
  });
  /** The spell the player concentrates on, written out. */
  protected readonly ownConcentration = computed(() => {
    return this.own()?.concentrationSpellNamePt ?? '';
  });
  /** The melee attacks an opportunity attack can use: off turn, with the reaction
   * free, someone in reach and the character on its feet (E6-28). */
  protected readonly opportunities = computed(() => {
    const own = this.own();
    const opts = this.options();
    if (this.isMaster() || !own || this.myTurn() || !this.active() || own.reactionUsed || isDown(own) || !opts?.options) {
      return [];
    }
    return opts.options.attacks.flatMap((a) => {
      const atk = a.attack;
      // The melee reach is 5 ft, whatever range the weapon has when thrown.
      const reach = atk
        ? opts.attackTargets.find((t) => t.attackKey === atk.key)?.targets.some((t) => t.distanceFt !== undefined && t.distanceFt <= 5)
        : false;
      return atk && atk.kind === AttackKind.WEAPON && atk.saveDc === 0 && atk.melee && reach
        ? [{ key: atk.key, name: atk.namePt || atk.name }]
        : [];
    });
  });
  /** The characters at three failures: the master is asked to confirm each death (E6-30). */
  protected readonly dying = computed(() =>
    this.isMaster() ? (this.encounter()?.combatants ?? []).filter((c) => c.state === CombatantState.DYING) : [],
  );
  /** The player's hit whose damage is still to roll (the sheet was closed). */
  protected readonly ownPendingRoll = computed(() => {
    const own = this.own();
    return (
      this.options()?.pendingDamages.find(
        (p) => p.status === PendingDamageStatus.AWAITING_ROLL && p.attackerId === own?.id,
      ) ?? null
    );
  });
  /** What the master's turn still owes a hit: "Falta aplicar 5 de dano". */
  protected readonly owed = computed(() => (this.isMaster() ? pendingNote(this.options()?.pendingDamages ?? []) : null));
  /** The master's card shows for an NPC on turn, and for any turn that left
   * damage to apply. */
  protected readonly masterCard = computed(() => {
    const c = this.subject();
    return this.isMaster() && c ? c : null;
  });
  /** The newest attack of the one on turn was stopped by the target's Escudo (from the log). */
  protected readonly reactionStopped = computed(() => {
    const who = this.subject();
    const entry = this.log
      .rounds()
      .flatMap((r) => r.entries)
      .find((x) => x.kind === CombatLogKind.ATTACK && x.actorId === who?.id);
    return !!entry?.stoppedByReaction;
  });
  protected readonly cardVisible = computed(() => {
    const c = this.masterCard();
    return (
      !!c &&
      (!isPlayer(c) || openDamages(this.options()?.pendingDamages ?? []).length > 0 || this.settledTurn() === c.id)
    );
  });
  protected readonly unplaced = computed(() =>
    (this.encounter()?.combatants ?? []).filter((c) => !c.placed),
  );
  protected readonly setup = computed(() => this.encounter()?.status === EncounterStatus.SETUP);
  protected readonly active = computed(() => this.encounter()?.status === EncounterStatus.ACTIVE);
  protected readonly ended = computed(() => this.encounter()?.status === EncounterStatus.ENDED);

  constructor() {
    // The master's lists need the roster (the kind of each NPC, the class
    // line, the player's name).
    effect(() => {
      if (this.isMaster() && this.campaignId()) {
        void this.loadRoster(this.campaignId());
      }
    });
    // The options are asked for again whenever the combat changes (every
    // call and every event bumps its revision) and whoever is in the
    // spotlight changes. A turn that ends leaves nothing to show.
    effect(() => {
      const e = this.encounter();
      const who = this.subject();
      const campaignId = this.campaignId();
      if (!e || !who) {
        untracked(() => this.turn.clear());
        return;
      }
      void e.revision;
      untracked(() => void this.turn.load(this.api, campaignId, e.id, who.id));
    });
    // The log is read again on every `combat_log_changed` and after every
    // reconnection (the page bumps `logTick`).
    effect(() => {
      const e = this.encounter();
      const campaignId = this.campaignId();
      this.state().logTick();
      if (!e || e.status === EncounterStatus.SETUP) {
        untracked(() => this.log.clear());
        return;
      }
      untracked(() => void this.log.load(this.api, campaignId, e.id));
    });
    // What a turn said is not carried into the next one.
    let turnOf = '';
    effect(() => {
      const id = this.encounter()?.currentCombatantId ?? '';
      if (id !== turnOf) {
        turnOf = id;
        untracked(() => {
          this.actionNote.set('');
          this.deathResult.set('');
        });
      }
    });
    // A hit on the player's character that waits for their reaction opens Escudo (E6-28).
    effect(() => {
      const e = this.encounter();
      const own = this.own();
      if (this.isMaster() || !e || !own || e.status !== EncounterStatus.ACTIVE) {
        return;
      }
      const prompt = e.reactionPrompts.find((p) => p.targetId === own.id);
      if (prompt && !this.shieldHandled.has(prompt.pendingDamageId)) {
        this.shieldHandled.add(prompt.pendingDamageId);
        untracked(() => this.openShield(prompt));
      }
    });
    // "Mover" belongs to the player's turn: when it ends, the page closes.
    effect(() => {
      if (!this.myTurn() || !this.active()) {
        this.state().moving.set(false);
      }
      if (!this.active()) {
        this.state().mapOpen.set(false);
      }
    });
  }

  private async loadRoster(campaignId: string): Promise<void> {
    try {
      const entries = await this.roster.list(campaignId);
      this.rosterInfo.set(
        new Map(
          entries.map((e) => [
            e.id,
            {
              classSummary: e.classSummary,
              playerName: e.playerName,
              kindLabel: e.kind === CharacterKind.PLAYER ? '' : npcKindLabel(e.kind),
              raceName: e.raceName,
            },
          ]),
        ),
      );
    } catch {
      // Best effort: the lists fall back to what the combat itself says.
    }
  }

  // ---- calls ----

  /** Runs a call that answers with the combat: applies it, or says why not.
   * A refusal that means the screen is stale reads the combat again. */
  private async run(call: (e: Encounter) => Promise<Encounter>): Promise<boolean> {
    const e = this.encounter();
    if (!e || this.busy()) {
      return false;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      this.state().apply(await call(e));
      return true;
    } catch (err) {
      this.error.set(combatErrorMessage(err));
      await this.refreshAfter(err);
      return false;
    } finally {
      this.busy.set(false);
    }
  }

  private async refreshAfter(err: unknown): Promise<void> {
    const code = ConnectError.from(err).code;
    if (code === Code.Aborted || code === Code.FailedPrecondition || code === Code.NotFound) {
      try {
        this.state().apply(await this.api.get(this.campaignId()));
      } catch {
        // The stream's next `ready` reads it again.
      }
    }
  }

  protected begin(): Promise<boolean> {
    return this.run((e) => this.api.begin(this.campaignId(), e.id));
  }

  /** `discard`: the master passes the turn although a damage waits. */
  protected nextTurn(discard = false): Promise<boolean> {
    return this.run((e) => this.api.endTurn(this.campaignId(), e.id, e.currentCombatantId, discard));
  }

  /** A standard action ("Disparada"): it spends the action; Dash doubles the
   * movement. */
  protected takeAction(key: string): Promise<boolean> {
    const own = this.own();
    return own
      ? this.run(async (e) => (await this.api.takeAction(this.campaignId(), e.id, own.id, key)).encounter)
      : Promise.resolve(false);
  }

  /** "Usar" on a feature: Retomar o Fôlego rolls (its own sheet); the others
   * only spend the action and remind the table ("Você tem outra ação"). */
  protected async useFeature(key: string): Promise<void> {
    const own = this.own();
    const e = this.encounter();
    const option = this.options()?.options?.featureActions.find((a) => a.action?.key === key);
    if (!own || !e || !option?.action) {
      return;
    }
    const name = option.action.namePt;
    if (key === 'feature:second-wind') {
      const data: FeatureSheetData = {
        campaignId: this.campaignId(),
        encounterId: e.id,
        combatantId: own.id,
        actionKey: key,
        name,
        cost: option.action.economy === ActionEconomy.BONUS_ACTION ? 'Ação bônus' : 'Ação',
        diceMode: this.diceMode(),
        preference: this.dicePreference(),
        state: this.state(),
      };
      openSheet<FeatureSheet, FeatureSheetData, boolean>(this.dialog, this.bottomSheet, FeatureSheet, {
        data,
        ariaLabel: name,
        labelledBy: 'sheet-t',
      }).subscribe();
      return;
    }
    const actionKey = key;
    // Surto de Ação (any level's feature) gives the action back.
    const note = option.action.resourceKey === 'action_surge' ? `${name}: você tem outra ação.` : `${name}: usado.`;
    if (await this.run(async (current) => (await this.api.takeAction(this.campaignId(), current.id, own.id, actionKey)).encounter)) {
      this.actionNote.set(note);
    }
  }

  /** "Atacar": the attack sheet (Alvo, Rolar, Dano). `asReaction` is the
   * opportunity attack, off turn. */
  protected openAttack(key: string, asReaction = false): void {
    this.attackSheet(key, undefined, asReaction);
  }

  /** "Rolar o dano" of a hit whose sheet was closed before the damage. */
  protected rollPendingDamage(): void {
    const p = this.ownPendingRoll();
    if (!p) {
      return;
    }
    const attack = this.options()?.options?.attacks.find((a) => a.attack?.key === p.attackKey)?.attack;
    if (attack && attack.saveDc === 0) {
      this.attackSheet(p.attackKey, p);
      return;
    }
    // A spell's damage (Mísseis Mágicos, Mãos Flamejantes): the cast sheet, at the damage.
    const own = this.own();
    const owed = (this.options()?.pendingDamages ?? []).filter(
      (x) => x.status === PendingDamageStatus.AWAITING_ROLL && x.attackerId === own?.id && x.attackKey === p.attackKey,
    );
    this.castSheet(p.attackKey, owed);
  }

  private attackSheet(key: string, resume?: PendingDamage, asReaction = false): void {
    const e = this.encounter();
    const own = this.own();
    const attack = this.options()?.options?.attacks.find((a) => a.attack?.key === key)?.attack;
    if (!e || !own || !attack) {
      return;
    }
    const data: AttackSheetData = {
      campaignId: this.campaignId(),
      encounterId: e.id,
      attackerId: own.id,
      round: e.round,
      attack,
      targets: this.options()?.attackTargets.find((t) => t.attackKey === key)?.targets ?? [],
      diceMode: this.diceMode(),
      preference: this.dicePreference(),
      state: this.state(),
      asReaction,
      attacksLeft: this.attacksLeft(),
      attacksPerAction: this.attacksPerAction(),
      resume: resume
        ? { pending: resume, targetLabel: e.combatants.find((c) => c.id === resume.targetId)?.label ?? '' }
        : undefined,
    };
    openSheet<AttackSheet, AttackSheetData, boolean>(this.dialog, this.bottomSheet, AttackSheet, {
      data,
      ariaLabel: asReaction ? `Ataque de oportunidade com ${attack.namePt || attack.name}` : `Atacar com ${attack.namePt || attack.name}`,
      labelledBy: 'sheet-t',
    }).subscribe();
  }

  /** "Conjurar": the cast sheet (the slot, the targets, the roll, the damage). */
  protected openCast(key: string): void {
    this.castSheet(key);
  }

  /** The cast sheet for a spell of the options, or a cantrip that asks for a
   * saving throw; with `resume` it opens at the damage the player never rolled. */
  private castSheet(key: string, resume?: readonly PendingDamage[]): void {
    const e = this.encounter();
    const own = this.own();
    const opts = this.options();
    if (!e || !own || !opts?.options) {
      return;
    }
    const spell = opts.options.spells.find((s) => s.spell?.key === key);
    const cantrip = opts.options.attacks.find((a) => a.attack?.key === key)?.attack;
    const name = spell?.spell ? spell.spell.namePt || spell.spell.name : cantrip ? cantrip.namePt || cantrip.name : key;
    const vitals = this.vitals().find((v) => v.characterId === own.characterId);
    const shield = opts.options.spells.find((s) => s.spell?.key === 'spell:shield');
    const attackBonus = this.spellAttackBonus(opts.options.attacks.flatMap((a) => (a.attack ? [a.attack] : [])));
    const data: CastSheetData = {
      campaignId: this.campaignId(),
      encounterId: e.id,
      casterId: own.id,
      round: e.round,
      spellKey: key,
      name,
      level: spell?.spell?.level ?? 0,
      concentration: spell?.spell?.concentration ?? false,
      economy: spell?.economy ?? ActionEconomy.ACTION,
      slots: spell?.slots ?? [],
      usage: vitals?.spellSlots ?? [],
      pact: vitals?.pactSlots ?? null,
      targets: opts.spellTargets.find((t) => t.spellKey === key),
      shieldFree: shield ? shield.slots.reduce((n, s) => n + s.free, 0) : null,
      shieldName: shield?.spell?.namePt || 'Escudo Arcano',
      attackBonus,
      diceMode: this.diceMode(),
      preference: this.dicePreference(),
      state: this.state(),
      resume,
    };
    openSheet<CastSheet, CastSheetData, boolean>(this.dialog, this.bottomSheet, CastSheet, {
      data,
      ariaLabel: resume ? `Rolar o dano de ${name}` : `Conjurar ${name}`,
      labelledBy: 'sheet-t',
    }).subscribe();
  }

  /** The spell attack bonus, for a typed d20: the one of a spell attack in the
   * options, or a save cantrip's DC minus 8 (the same proficiency and modifier). */
  private spellAttackBonus(attacks: readonly Attack[]): number {
    const spellAttack = attacks.find((a) => a.kind === AttackKind.SPELL && a.saveDc === 0);
    if (spellAttack) {
      return spellAttack.attackBonus;
    }
    const save = attacks.find((a) => a.saveDc > 0);
    return save ? save.saveDc - 8 : 0;
  }

  /** The death save of the player's own turn (RN-03, RN-18 per roll). */
  protected async rollDeathSave(die: AttackDie): Promise<void> {
    const own = this.own();
    if (!own) {
      return;
    }
    const key = this.deathKey;
    await this.run(async (e) => {
      const res = await this.api.rollDeathSave(this.campaignId(), e.id, own.id, die, key);
      this.deathKey = newKey();
      const text = saveAnnouncement(res.save, own.label);
      this.deathResult.set(text);
      if (res.save.outcome === DeathSaveOutcome.REVIVED) {
        this.actionNote.set(text);
      }
      return res.encounter;
    });
  }

  /** The master's "Confirmar a morte" (ConfirmDeath): the character is dead for good. */
  protected async confirmDeath(id: string): Promise<void> {
    await this.run((e) => this.api.confirmDeath(this.campaignId(), e.id, id, newKey()));
  }

  protected deathLaterFor(id: string): void {
    this.deathLater.update((s) => new Set(s).add(id));
  }

  protected deathAskAgain(id: string): void {
    this.deathLater.update((s) => {
      const next = new Set(s);
      next.delete(id);
      return next;
    });
  }

  /** The player ends their own concentration (the conditions are the master's). */
  protected endOwnConcentration(): Promise<boolean> {
    const own = this.own();
    return own
      ? this.run((e) => this.api.setConditions(this.campaignId(), e.id, own.id, { endConcentration: true }))
      : Promise.resolve(false);
  }

  /** "Condições…" in a combatant's menu (the master). */
  protected openConditions(combatantId: string): void {
    const e = this.encounter();
    const combatant = e?.combatants.find((c) => c.id === combatantId);
    if (!e || !combatant) {
      return;
    }
    const data: ConditionsData = { campaignId: this.campaignId(), encounterId: e.id, combatant, state: this.state() };
    openSheet<ConditionsDialog, ConditionsData, boolean>(this.dialog, this.bottomSheet, ConditionsDialog, {
      data,
      ariaLabel: `Condições de ${combatant.label}`,
      labelledBy: 'sheet-t',
      width: '600px',
    }).subscribe();
  }

  /** "Você foi atingido: usar Escudo?": opens by itself, and has to be answered. */
  private openShield(prompt: ReactionPrompt): void {
    const e = this.encounter();
    const own = this.own();
    const vitals = this.vitals().find((v) => v.characterId === own?.characterId);
    if (!e) {
      return;
    }
    const data: ShieldSheetData = {
      campaignId: this.campaignId(),
      encounterId: e.id,
      prompt,
      round: e.round,
      usage: vitals?.spellSlots ?? [],
      pact: vitals?.pactSlots ?? null,
      state: this.state(),
      armorClass: this.armorClass(),
    };
    openSheet<ShieldSheet, ShieldSheetData, boolean>(this.dialog, this.bottomSheet, ShieldSheet, {
      data,
      ariaLabel: 'Você foi atingido: usar Escudo Arcano?',
      alert: true,
    }).subscribe();
  }

  /** A damage was settled: the master's card stays for its note. */
  protected settled(): void {
    this.settledTurn.set(this.subject()?.id ?? '');
  }

  /** "Dano/Cura" on an NPC (`AdjustCombatantHitPoints`). */
  protected adjustNpc(combatantId: string): void {
    const e = this.encounter();
    const combatant = e?.combatants.find((c) => c.id === combatantId);
    if (!e || !combatant) {
      return;
    }
    openSheet<AdjustNpc, AdjustNpcData, boolean>(this.dialog, this.bottomSheet, AdjustNpc, {
      data: { campaignId: this.campaignId(), encounterId: e.id, combatant, state: this.state() },
      ariaLabel: `Dano ou cura em ${combatant.label}`,
      labelledBy: 'adjust-npc-title',
      width: '440px',
    }).subscribe();
  }

  protected endCombat(): Promise<boolean> {
    return this.run((e) => this.api.end(this.campaignId(), e.id));
  }

  protected submitFace(combatantId: string, face: number): Promise<boolean> {
    return this.run((e) => this.api.submitInitiative(this.campaignId(), e.id, combatantId, { face }));
  }

  protected rollInApp(combatantId: string): Promise<boolean> {
    return this.run((e) => this.api.submitInitiative(this.campaignId(), e.id, combatantId, { inApp: true }));
  }

  protected setOrder(ids: string[]): Promise<boolean> {
    return this.run((e) => this.api.setOrder(this.campaignId(), e.id, ids));
  }

  protected reveal(change: { id: string; hidden: boolean }): Promise<boolean> {
    return this.run((e) => this.api.setHidden(this.campaignId(), e.id, change.id, change.hidden));
  }

  protected remove(id: string): Promise<boolean> {
    return this.run((e) => this.api.remove(this.campaignId(), e.id, id));
  }

  protected adjustFor(characterId: string): void {
    const v = this.vitals().find((x) => x.characterId === characterId);
    if (v) {
      this.adjust.emit(v);
    }
  }

  /** "Adicionar combatente": the start dialog, with the NPCs only. */
  protected addCombatants(): void {
    const e = this.encounter();
    if (!e) {
      return;
    }
    const phone = this.phone();
    this.dialog
      .open<StartCombatDialog, StartCombatData, Encounter>(StartCombatDialog, {
        data: {
          campaignId: this.campaignId(),
          mode: 'add',
          map: null,
          encounterId: e.id,
          existing: e.combatants.length,
        },
        width: phone ? '100vw' : '760px',
        maxWidth: phone ? '100vw' : 'calc(100vw - 32px)',
        height: phone ? '100dvh' : undefined,
        maxHeight: phone ? '100dvh' : '92dvh',
        ariaLabelledBy: 'start-title',
        autoFocus: 'first-tabbable',
      })
      .afterClosed()
      .subscribe((added) => added && this.state().apply(added));
  }

  // ---- moving ----

  /** A token dropped on a square (the master's drag, or a player's inside
   * their reach). The token is where it was dropped at once; if the server
   * refuses, it goes back and says why. */
  protected async drop(drop: TokenDrop): Promise<void> {
    const e = this.encounter();
    const c = e?.combatants.find((x) => x.id === drop.id);
    if (!e || !c) {
      return;
    }
    const from: Square = { col: c.col, row: c.row };
    if (c.placed && from.col === drop.col && from.row === drop.row) {
      return;
    }
    // A player is checked here first: a drop beyond the reach goes nowhere.
    if (!this.isMaster() && !this.reachable(c.id, drop)) {
      this.error.set('Longe demais ou ocupado: solte o token num quadrado destacado.');
      return;
    }
    this.error.set('');
    this.state().applyMove({ encounterId: e.id, combatantId: c.id, col: drop.col, row: drop.row });
    // One save at a time per combatant: `MoveSaves` is about positions, and
    // here a square's column and row stand in for x and y.
    await this.moves.move(
      `${e.id}/${c.id}`,
      { xBp: from.col, yBp: from.row },
      { xBp: drop.col, yBp: drop.row },
      {
        save: async (to) =>
          this.state().apply(await this.api.move(this.campaignId(), e.id, c.id, to.xBp, to.yBp)),
        failed: (saved, err) => {
          this.state().applyMove({ encounterId: e.id, combatantId: c.id, col: saved.xBp, row: saved.yBp });
          this.error.set(combatErrorMessage(err, 'mover o token'));
          void this.refreshAfter(err);
        },
      },
    );
  }

  private reachable(id: string, to: Square): boolean {
    const e = this.encounter();
    const c = e?.combatants.find((x) => x.id === id);
    if (!e || !c) {
      return false;
    }
    const others = e.combatants.filter((x) => x.placed && x.id !== id).map((x) => ({ col: x.col, row: x.row }));
    return canReach({ col: c.col, row: c.row }, to, reachSquares(c.movementLeftFt), e.gridColumns, e.gridRows, others);
  }

  /** "Mover para cá" on the "Mover" page. */
  protected async confirmMove(to: Square): Promise<void> {
    const own = this.own();
    if (!own) {
      return;
    }
    this.moveError.set('');
    const ok = await this.run((e) => this.api.move(this.campaignId(), e.id, own.id, to.col, to.row));
    if (ok) {
      this.state().moving.set(false);
    } else {
      // The refusal's own words, next to the map where the person looks.
      this.moveError.set(this.error());
      this.error.set('');
    }
  }

  protected closeFullPage(): void {
    this.state().moving.set(false);
    this.state().mapOpen.set(false);
  }

  protected openMove(): void {
    this.moveError.set('');
    this.state().moving.set(true);
  }

  /** The first free square nearest the middle of the map: where "Colocar no
   * mapa" puts a combatant that came without a token. */
  protected async place(id: string): Promise<void> {
    const e = this.encounter();
    if (!e) {
      return;
    }
    const taken = new Set(e.combatants.filter((c) => c.placed).map((c) => `${c.col},${c.row}`));
    const middle: Square = { col: Math.floor(e.gridColumns / 2), row: Math.floor(e.gridRows / 2) };
    let best: Square | null = null;
    for (let row = 0; row < e.gridRows; row++) {
      for (let col = 0; col < e.gridColumns; col++) {
        const d = Math.max(Math.abs(col - middle.col), Math.abs(row - middle.row));
        const dBest = best ? Math.max(Math.abs(best.col - middle.col), Math.abs(best.row - middle.row)) : Infinity;
        if (!taken.has(`${col},${row}`) && d < dBest) {
          best = { col, row };
        }
      }
    }
    if (best) {
      const spot = best;
      await this.run((current) => this.api.move(this.campaignId(), current.id, id, spot.col, spot.row));
    }
  }

  protected leave(): void {
    this.state().dismissEnded();
  }
}

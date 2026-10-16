import { ElementRef, Injector, inject, input, output, Directive } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatDialog } from '@angular/material/dialog';
import type { DiceMode, DicePreference } from '../../../../gen/meurpg/campaigns/v1/campaigns_pb';
import { CombatClient } from '../../../core/combat/combat-client';
import { CombatUndone } from '../../../core/combat/combat-undone';
import { TableRulesClient } from '../../../core/campaigns/table-rules';
import type { FogView } from '../../../core/maps/fog-view';
import { MapsClient } from '../../../core/maps/maps-client';
import { type SearchSkills } from '../../../core/traps/trap-search';
import type { TrapBoard } from '../../../core/traps/trap-board';
import type { CombatState } from '../../../core/combat/combat-state';
import { SpellCatalog } from '../../../core/combat/spell-catalog';
import { RollAnimator } from '../../../shared/roll-overlay/roll-animator';
import type { MapState } from '../../../core/maps/map-state';
import { RosterClient } from '../../../core/maps/roster-client';
import { TurnAlert } from '../../../core/combat/turn-alert';
import { PHONE_QUERY, mediaQuery } from '../../../shared/map-view/media-query';
import type { PartyMemberInfoVm, VitalsVm } from '../../../core/play/live-session.types';
import { CreaturesClient } from '../../../core/creatures/creatures-client';

/**
 * What the combat screen is handed (its inputs and outputs) and the services it builds on. The base of the layers that
 * make up `CombatView` (`combat-view-state.ts`, `combat-view-derived.ts`, `combat-view-parts.ts`,
 * `combat-view-flows.ts`): each file holds one kind of member and the next one extends it, so the template and the specs
 * keep reading every member from the one component.
 */
@Directive()
export abstract class CombatViewInputsDirective {
  protected readonly api = inject(CombatClient);
  protected readonly animator = inject(RollAnimator);
  protected readonly undone = inject(CombatUndone);
  protected readonly roster = inject(RosterClient);
  protected readonly maps = inject(MapsClient);
  protected readonly catalog = inject(SpellCatalog);
  protected readonly dialog = inject(MatDialog);
  protected readonly creaturesApi = inject(CreaturesClient);
  protected readonly tableRules = inject(TableRulesClient);
  protected readonly bottomSheet = inject(MatBottomSheet);
  protected readonly turnAlert = inject(TurnAlert);
  protected readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  protected readonly injector = inject(Injector);
  protected readonly phone = mediaQuery(PHONE_QUERY);
  /** From 1024px the master has the combat bar (E6-11); below it the turn card does it all (E6-12). */
  protected readonly laptop = mediaQuery('(min-width: 1024px)');
  /** From 1280px the player's order is a column on the left (E6-14). */
  protected readonly wideLayout = mediaQuery('(min-width: 1280px)');
  readonly campaignId = input.required<string>();
  readonly isMaster = input(false);
  readonly state = input.required<CombatState>();
  readonly mapState = input.required<MapState>();
  /** What the player sees of the map with the fog on (MR-036): the combat map draws it, and its layers are the ones filtered to it. */
  readonly fog = input<FogView | null>(null);
  readonly vitals = input<readonly VitalsVm[]>([]);
  readonly partyInfo = input<ReadonlyMap<string, PartyMemberInfoVm>>(new Map());
  readonly sessionNumber = input(0);
  /** The players' "O combate acabou" card is above: the summary does not repeat its heading on screen. */
  readonly cardAbove = input(false);
  /** The player's own armor class from their sheet (without Escudo's +5), for the Escudo result. */
  readonly armorClass = input<number | null>(null);
  /** "Clérigo 5": the player's class line, for the Revivificar sheet's header. */
  readonly casterClasses = input('');
  readonly diceMode = input.required<DiceMode>();
  readonly dicePreference = input.required<DicePreference>();
  /** The player's bonuses in Percepção and Investigação, for "Procurar" (E9-08); `null` while unknown. */
  readonly trapSkills = input<SearchSkills | null>(null);
  /** The traps' board: the master's damage that waits for him (a trap that fired in the combat), read here too. */
  readonly traps = input<TrapBoard | null>(null);

  /** "Dano/Cura" on a player's row: the page opens its adjust dialog. */
  readonly adjust = output<VitalsVm>();
  /** "Mudar" (how the player rolls). */
  readonly changeDice = output<void>();
}

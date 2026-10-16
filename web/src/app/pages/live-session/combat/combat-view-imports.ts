import { NgTemplateOutlet } from '@angular/common';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { CheckRow } from '../../../shared/form-fields/check-row';
import { ActionGroups } from './action-groups/action-groups';
import { DeathQuestion } from './death-question/death-question';
import { DeathSaves } from './death-saves/death-saves';
import { DeathSaveDue } from './death-save-due/death-save-due';
import { EffectsPanel } from '../effects/effects-panel';
import { HitRiderCard } from './hit-rider/hit-rider-card';
import { RageQuestion } from './rage-end/rage-question';
import { InspirationCard } from './inspiration-card/inspiration-card';
import { ContestMaster } from './contest-master/contest-master';
import { ReactionQueue } from './reaction-queue/reaction-queue';
import { SurpriseCard } from './surprise-card/surprise-card';
import { CombatLogPanel } from './combat-log/combat-log-panel';
import { CombatBar } from './combat-bar/combat-bar';
import { CombatMapCard } from './combat-map-card/combat-map-card';
import { CombatSummary } from './combat-summary/combat-summary';
import { JointCard } from './joint-turn/joint-card';
import { InitiativeSetup } from './initiative-setup/initiative-setup';
import { InitiativeSide } from './initiative-side/initiative-side';
import { MovePage } from './move-page/move-page';
import { OpportunityCard } from './opportunity/opportunity-card';
import { HiddenRevealCard } from './hidden-reveal/hidden-reveal-card';
import { NpcCard } from './npc-card/npc-card';
import { DeathsBlock } from './deaths-block/deaths-block';
import { RollModeQueue } from './roll-mode/roll-mode-queue';
import { OrderList } from './order-list/order-list';
import { PlayerInitiative } from './player-initiative/player-initiative';
import { BeastTraits } from '../../../shared/wild-shape/beast-traits';
import { CreatureBar } from './creature-turn/creature-bar';
import { CreatureBlock } from './creature-turn/creature-block';
import { CreatureHero } from './creature-turn/creature-hero';
import { MineTabs } from './mine-tabs/mine-tabs';
import { TrapDamages } from '../traps/trap-damages/trap-damages';
import { OrderColumn } from './turn-panel/order-column';
import { OrderStrip } from './turn-panel/order-strip';
import { TurnBar } from './turn-panel/turn-bar';
import { TurnPanel } from './turn-panel/turn-panel';
import { CoverPanel } from './theatre/cover-panel';
import { NoMapPanel, TheatreReaction } from './theatre/no-map-panel';
import { OfferPanel } from './theatre/offer-panel';

/** The components, directives and pipes the combat screen's template uses. */
export const COMBAT_VIEW_IMPORTS = [
  CheckRow,
  ActionGroups,
  BeastTraits,
  CreatureBar,
  CreatureBlock,
  CreatureHero,
  DeathQuestion,
  DeathSaves,
  DeathSaveDue,
  EffectsPanel,
  RageQuestion,
  HitRiderCard,
  InspirationCard,
  CombatBar,
  HiddenRevealCard,
  CombatLogPanel,
  CombatMapCard,
  CombatSummary,
  CoverPanel,
  InitiativeSetup,
  InitiativeSide,
  JointCard,
  MatButtonModule,
  MatIconModule,
  MineTabs,
  MovePage,
  NgTemplateOutlet,
  NoMapPanel,
  NpcCard,
  OfferPanel,
  OpportunityCard,
  OrderColumn,
  DeathsBlock,
  OrderStrip,
  OrderList,
  PlayerInitiative,
  RollModeQueue,
  TheatreReaction,
  ReactionQueue,
  ContestMaster,
  SurpriseCard,
  TurnBar,
  TrapDamages,
  TurnPanel,
];

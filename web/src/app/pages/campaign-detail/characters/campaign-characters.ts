import { Component, OnInit, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MatMenuModule } from '@angular/material/menu';
import { RouterLink } from '@angular/router';

import { describeCharacterError } from '../../../core/characters/character-errors';
import {
  characterKindLabel,
  characterStateLabel,
} from '../../../core/characters/character-labels';
import { CharacterKind } from '../../../core/characters/characters.types';
import {
  CampaignCharacterListItemVm,
  CampaignCharactersSource,
  CampaignCharactersVm,
} from './campaign-characters.types';

type ListState =
  | { status: 'loading' }
  | { status: 'ready'; vm: CampaignCharactersVm }
  | { status: 'error'; message: string };

/** The four NPC kinds a master can create, and the `:tipo` route segment
 * each maps to (plan §5: `/campanhas/:id/npcs/novo/:tipo`). */
const NPC_KINDS: ReadonlyArray<{ kind: CharacterKind; tipo: string }> = [
  { kind: 'enemy', tipo: 'inimigo' },
  { kind: 'boss', tipo: 'boss' },
  { kind: 'minion', tipo: 'minion' },
  { kind: 'story', tipo: 'historia' },
];

/**
 * The "Personagens" section on `/campanhas/:id` (MR-003, MR-005, MR-024).
 *
 * - **Master:** "Esperando aprovação" (characters created through an invite
 *   that requires approval, RN-15 — each links to its sheet, where the
 *   master approves or rejects it), "Personagens dos jogadores" (name,
 *   player display name, class/level summary, state chip) and "NPCs" (name,
 *   kind chip), plus a "Novo NPC" menu.
 * - **Player:** their own characters (a pending one shows "Pendente de
 *   aprovação"), and a "Criar meu personagem" call to action shown only
 *   when they have no living character (RN-03; a pending one counts).
 */
@Component({
  selector: 'app-campaign-characters',
  imports: [MatButtonModule, MatChipsModule, MatMenuModule, RouterLink],
  templateUrl: './campaign-characters.html',
  styleUrl: './campaign-characters.scss',
})
export class CampaignCharacters implements OnInit {
  private readonly source = inject(CampaignCharactersSource);

  readonly campaignId = input.required<string>();
  readonly isMaster = input.required<boolean>();

  protected readonly state = signal<ListState>({ status: 'loading' });
  protected readonly characterKindLabel = characterKindLabel;
  protected readonly characterStateLabel = characterStateLabel;
  protected readonly npcKinds = NPC_KINDS;

  /** Master only: the players' characters that wait for approval. */
  protected awaitingApproval(vm: CampaignCharactersVm): readonly CampaignCharacterListItemVm[] {
    return vm.playerCharacters.filter((c) => c.state === 'pending');
  }

  /** The master's "Personagens dos jogadores": everything already in the
   * campaign. A player sees all of their own, pending included. */
  protected listedPlayerCharacters(vm: CampaignCharactersVm): readonly CampaignCharacterListItemVm[] {
    return this.isMaster() ? vm.playerCharacters.filter((c) => c.state !== 'pending') : vm.playerCharacters;
  }

  ngOnInit(): void {
    // Not the constructor — see CampaignInvites's doc comment on the same
    // pattern: a required signal input is only set before the first
    // change-detection pass, and ngOnInit is guaranteed to run after that.
    this.load();
  }

  private load(): void {
    this.state.set({ status: 'loading' });
    this.source.listCharacters(this.campaignId()).then(
      (vm) => this.state.set({ status: 'ready', vm }),
      (err: unknown) => this.state.set({ status: 'error', message: describeCharacterError(err) }),
    );
  }
}

import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  OnInit,
  afterNextRender,
  computed,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { RouterLink } from '@angular/router';

import { characterKindLabel, characterStateLabel } from '../../../core/characters/character-labels';
import { CharacterKind } from '../../../core/characters/characters.types';
import { PanelAction } from '../panel-action/panel-action';
import { PanelHead } from '../panel-head/panel-head';
import { characterRowSub } from './campaign-characters.copy';
import { CampaignCharacterListItemVm } from './campaign-characters.types';
import { CampaignRoster } from './campaign-roster';

/** The four NPC kinds a master can create, and the `:kind` route segment
 * each maps to (plan §5: `/campaigns/:id/npcs/new/:kind`). */
const NPC_KINDS: ReadonlyArray<{ kind: CharacterKind; segment: string }> = [
  { kind: 'enemy', segment: 'enemy' },
  { kind: 'boss', segment: 'boss' },
  { kind: 'minion', segment: 'minion' },
  { kind: 'story', segment: 'story' },
];

/** How many NPCs show before "Ver todos os N NPCs". */
const SHOWN = 5;

/**
 * The master's "NPCs" panel (MR-005; board C6-02) on "Preparação": the count and the "Novo NPC"
 * menu in the title row, then the NPCs (initials, name, what it is), each linking to its sheet;
 * five show, then "Ver todos os N NPCs". It reads the page's `CampaignRoster`, the list the
 * players' panel on "Mesa" reads too.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-campaign-npcs',
  imports: [MatIconModule, MatMenuModule, PanelAction, PanelHead, RouterLink],
  templateUrl: './campaign-npcs.html',
  styleUrl: './campaign-npcs.scss',
})
export class CampaignNpcs implements OnInit {
  protected readonly roster = inject(CampaignRoster);

  readonly campaignId = input.required<string>();

  protected readonly characterKindLabel = characterKindLabel;
  protected readonly characterStateLabel = characterStateLabel;
  protected readonly characterRowSub = characterRowSub;
  protected readonly npcKinds = NPC_KINDS;
  protected readonly shown = SHOWN;
  protected readonly expanded = signal(false);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly npcs = computed(() => {
    const s = this.roster.state();
    return s.status === 'ready' ? s.vm.npcs : [];
  });
  protected readonly rows = computed(() =>
    this.expanded() ? this.npcs() : this.npcs().slice(0, SHOWN),
  );
  /** "9 NPCs", empty while the list is not here. */
  protected readonly count = computed(() =>
    this.roster.state().status === 'ready' ? `${this.npcs().length} NPCs` : '',
  );

  /** The two letters in the NPC's square: "CB" for Capitão bandido. */
  protected initials(name: string): string {
    const words = name.split(/\s+/).filter((w) => w !== '');
    const first = words[0]?.charAt(0) ?? '';
    return (words.length > 1 ? first + (words[1].charAt(0) ?? '') : first).toUpperCase();
  }

  /** What the NPC is, under the name: its class line, else its kind ("Inimigo"). */
  protected sub(c: CampaignCharacterListItemVm): string {
    return characterRowSub(c, true) || characterKindLabel(c.kind);
  }

  protected showAll(): void {
    this.expanded.set(true);
    afterNextRender(
      () => this.host.nativeElement.querySelector<HTMLElement>('.js-first-extra')?.focus(),
      { injector: this.injector },
    );
  }

  ngOnInit(): void {
    void this.roster.ensureLoaded(this.campaignId());
  }
}

import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, RouterLink } from '@angular/router';

import { CharacterKind } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { CreatureSummary } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { type BestiaryAccess, BestiaryAccessCheck } from '../../../core/creatures/bestiary-access';
import { creatureSlug } from '../../../core/creatures/bestiary-format';
import { EncounterDraft, ENTRY_MAX, KINDS_MAX, PARTY_NPC_MAX } from '../../../core/encounters/encounter-draft';
import { EncountersClient } from '../../../core/encounters/encounters-client';
import { GUIDE_CAVEAT, GUIDE_LABEL, capLine, headline, warningLines } from '../../../core/encounters/encounter-text';
import { formatInt, tight } from '../../../core/format/text';
import { RosterClient } from '../../../core/maps/roster-client';
import { CountStepper } from '../../../shared/count-stepper/count-stepper';
import { CreatureArt } from '../../../shared/creatures/creature-art';
import { openSheet } from '../../live-session/combat/sheet-host';
import { BudgetBar } from '../budget-bar/budget-bar';
import { PartyChips } from '../party-chips/party-chips';
import { CreaturePick } from '../creature-pick/creature-pick';
import { GenerateSheet, type GenerateData, type GenerateResult } from '../generate-sheet/generate-sheet';
import { PartyNpcSheet, type PartyNpcData } from '../party-npc-sheet/party-npc-sheet';
import { SaveSheet, type SaveData, type SaveResult } from '../save-sheet/save-sheet';
import type { DraftNpc } from '../../../core/encounters/encounter-draft';

type PageState = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready' };

/**
 * "/campanhas/:id/encontros" (MR-043, RN-29, E10-09): the master's encounter builder. The party (the living player
 * characters, plus NPCs he adds with a level), the bar of the difficulty, the creatures and their counts, "Gerar encontro"
 * and "Guardar no ponto de batalha". The browser does no maths: every change asks the server to measure the encounter
 * (`EncounterDraft`: debounced, one at a time, stale answers dropped) and the page draws the budgets, the band, the total
 * and the warnings it answers. Past the high budget the band is "Acima de alta": allowed, warned, never "mortal".
 *
 * Only the master gets the page: the builder and a saved encounter are his secret (RN-10), and the server answers a player
 * with `not_found`. `?mapa=&ponto=` (from a battle point of the map editor) opens "Guardar" on that point.
 */
@Component({
  selector: 'app-encounter-builder',
  imports: [BudgetBar, CountStepper, CreatureArt, CreaturePick, PartyChips, MatButtonModule, MatIconModule, MatProgressSpinnerModule, RouterLink],
  templateUrl: './encounter-builder.html',
  styleUrl: './encounter-builder.scss',
})
export class EncounterBuilder {
  private readonly route = inject(ActivatedRoute);
  private readonly accessCheck = inject(BestiaryAccessCheck);
  private readonly roster = inject(RosterClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);

  protected readonly campaignId = this.route.snapshot.paramMap.get('id') ?? '';
  protected readonly draft = new EncounterDraft(inject(EncountersClient), this.campaignId);
  protected readonly access = signal<BestiaryAccess | { status: 'loading' }>({ status: 'loading' });
  protected readonly state = signal<PageState>({ status: 'loading' });
  /** What the last "Guardar" kept, for the line above the page. */
  protected readonly saved = signal<SaveResult | null>(null);
  protected readonly npcError = signal('');

  protected readonly guide = GUIDE_LABEL;
  protected readonly caveat = GUIDE_CAVEAT;
  protected readonly format = formatInt;
  protected readonly slug = creatureSlug;
  protected readonly entryMax = ENTRY_MAX;

  protected readonly ev = this.draft.evaluation;
  protected readonly head = computed(() => (this.ev() ? headline(this.ev()!) : ''));
  protected readonly cap = computed(() => (this.ev() ? capLine(this.ev()!) : ''));
  protected readonly warnings = computed(() => (this.ev() ? warningLines(this.ev()!) : []));
  protected readonly total = computed(() => tight(`${formatInt(this.ev()?.totalXp ?? 0)} XP`));
  protected readonly creatures = computed(() => {
    const n = this.draft.creatureCount();
    return n === 1 ? '1 criatura' : `${n} criaturas`;
  });
  protected readonly canAddKind = computed(() => this.draft.entries().length < KINDS_MAX);
  protected readonly canAddNpc = computed(() => this.draft.npcs().length < PARTY_NPC_MAX);
  /** What each row shows: the draft's own count at once, the XP the server measured for it (dimmed while a newer measure is on its way). */
  protected readonly rows = computed(() => {
    const lines = new Map((this.ev()?.lines ?? []).map((l) => [l.creature?.key, l]));
    return this.draft.entries().map((e) => {
      const line = lines.get(e.creature.key);
      const fresh = line?.count === e.count;
      return {
        creature: e.creature,
        count: e.count,
        each: tight(`${formatInt(e.creature.xp)} XP cada`),
        subtotal: fresh ? tight(`${formatInt(line!.subtotalXp)} XP`) : '…',
        aboveCap: fresh && line!.aboveCap,
      };
    });
  });
  /** The chips of the party: the server's list (player characters first, then the NPCs in the order added). */
  protected readonly party = computed(() => {
    const members = this.ev()?.party ?? [];
    let npcIndex = -1;
    return members.map((m) => {
      if (m.npc) {
        npcIndex++;
      }
      return { name: m.name, level: m.level, npc: m.npc, index: m.npc ? npcIndex : -1 };
    });
  });

  constructor() {
    inject(DestroyRef).onDestroy(() => this.draft.stop());
    void this.start();
    const query = this.route.snapshot.queryParamMap;
    this.fromPoint = { mapId: query.get('mapa') ?? '', pointId: query.get('ponto') ?? '' };
  }

  private readonly fromPoint: { mapId: string; pointId: string };

  protected async start(): Promise<void> {
    this.access.set({ status: 'loading' });
    const access = await this.accessCheck.check(this.campaignId);
    this.access.set(access);
    if (access.status === 'master') {
      this.state.set({ status: 'ready' });
      this.draft.measureNow();
    }
  }

  protected setCount(key: string, count: number): void {
    this.saved.set(null);
    this.draft.setCount(key, count);
  }

  protected add(creature: CreatureSummary): void {
    this.saved.set(null);
    this.draft.add(creature);
  }

  protected async openNpc(): Promise<void> {
    this.npcError.set('');
    let npcs;
    try {
      const have = new Set(this.draft.npcs().map((n) => n.characterId));
      npcs = (await this.roster.list(this.campaignId)).filter((e) => e.kind !== CharacterKind.PLAYER && !have.has(e.id));
    } catch {
      this.npcError.set('Não deu para ler os NPCs da campanha: o servidor não respondeu. Tente de novo.');
      return;
    }
    openSheet<PartyNpcSheet, PartyNpcData, DraftNpc>(this.dialog, this.bottomSheet, PartyNpcSheet, {
      data: { campaignId: this.campaignId, npcs, entries: this.draft.specs(), party: this.draft.party(), before: this.ev() },
      ariaLabel: 'Pôr um NPC no grupo',
      labelledBy: 'pn-t',
      width: '620px',
      tall: true,
      focus: 'input[type=radio]:checked',
    }).subscribe((npc) => {
      if (npc) {
        this.saved.set(null);
        this.draft.addNpc(npc);
      }
    });
  }

  protected removeNpc(index: number): void {
    this.saved.set(null);
    this.draft.removeNpc(index);
  }

  protected openGenerate(): void {
    openSheet<GenerateSheet, GenerateData, GenerateResult>(this.dialog, this.bottomSheet, GenerateSheet, {
      data: {
        campaignId: this.campaignId,
        campaignName: this.access().status === 'master' ? (this.access() as { campaignName: string }).campaignName : '',
        party: this.draft.party(),
        evaluation: this.ev(),
      },
      ariaLabel: 'Gerar encontro',
      labelledBy: 'gen-t',
      width: '760px',
      tall: true,
      focus: 'input[type=radio]:checked',
    }).subscribe((result) => {
      if (result) {
        this.saved.set(null);
        this.draft.replace(result.entries, result.seed);
      }
    });
  }

  protected openSave(): void {
    const data: SaveData = {
      campaignId: this.campaignId,
      entries: this.draft.specs(),
      party: this.draft.party(),
      evaluation: this.ev(),
      mapId: this.fromPoint.mapId,
      pointId: this.fromPoint.pointId,
    };
    openSheet<SaveSheet, SaveData, SaveResult>(this.dialog, this.bottomSheet, SaveSheet, {
      data,
      ariaLabel: 'Guardar no ponto de batalha',
      labelledBy: 'save-t',
      width: '680px',
      tall: true,
      focus: 'select',
    }).subscribe((result) => {
      if (result) {
        this.saved.set(result);
      }
    });
  }

  /** The master's page is also where a lost read is retried. */
  protected retry(): void {
    this.draft.measureNow();
  }

  protected readonly savedLine = computed(() => {
    const s = this.saved();
    return s?.evaluation ? `${headline(s.evaluation)} · ${s.evaluation.creatureCount} criaturas` : '';
  });
}

import { Component, DestroyRef, ElementRef, Injector, afterNextRender, computed, effect, inject, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';

import { type CharacterCreature, CreatureSource } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { Creature } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { CharacterSheetSource } from '../character-sheet.types';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { creatureErrorMessage } from '../../../core/creatures/creature-errors';
import { sourcePhrase } from '../../../core/creatures/creature-format';
import { joinDots } from '../../../core/format/text';
import { CreatureArt } from '../../../shared/creatures/creature-art';
import { StatBlock } from '../../../shared/creatures/stat-block';
import { CreatureEdit, type EditMode } from '../creatures-panel/creature-edit';

type PageState =
  | { status: 'loading' }
  | { status: 'not-found' }
  | { status: 'error'; message: string }
  | { status: 'ready'; creature: CharacterCreature; block: Creature; ownerName: string; isMaster: boolean };

/**
 * "/campanhas/:id/personagens/:characterId/criaturas/:creatureId" (E9-10,
 * quadro 3): a creature's stat block as a page of its own. Read-only, in the
 * language of the paper sheet: the name the table gave it, the kind, its size
 * and where it came from; CA, PV (its own, "1 de 1"), the speeds, the six
 * abilities, then the lines and the book's traits and actions in English (see
 * `StatBlock`). A familiar does not attack (SRD): the page shows the book's
 * action with no "Atacar" and says why in one line.
 *
 * "Dispensar" asks in place and takes the person back to the sheet; the
 * player dismisses only what they conjured, the master any. "Renomear" is in
 * place too. RN-20: only the owner's player and the master read this; for
 * anyone else the server says `not_found`.
 */
@Component({
  selector: 'app-creature-page',
  imports: [CreatureArt, CreatureEdit, MatIconModule, MatProgressSpinnerModule, RouterLink, StatBlock],
  templateUrl: './creature-page.html',
  styleUrl: './creature-page.scss',
})
export class CreaturePage {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly client = inject(CreaturesClient);
  private readonly sheets = inject(CharacterSheetSource);
  private readonly destroyRef = inject(DestroyRef);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  protected readonly campaignId = signal('');
  protected readonly characterId = signal('');
  private readonly creatureId = signal('');
  protected readonly state = signal<PageState>({ status: 'loading' });
  protected readonly mode = signal<EditMode | null>(null);

  protected readonly back = computed(() => ['/campanhas', this.campaignId(), 'personagens', this.characterId()]);

  protected readonly subtitle = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    const summary = s.block.summary;
    return joinDots([s.creature.monsterNamePt, [summary?.sizePt, summary?.typePt].filter((p) => p).join(', ')].filter((p) => p));
  });
  protected readonly origin = computed(() => {
    const s = this.state();
    return s.status === 'ready' ? sourcePhrase(s.creature.source, s.ownerName) : '';
  });
  /** Why there is no "Atacar": what the server decided for this creature. */
  protected readonly attackNote = computed(() => {
    const s = this.state();
    if (s.status !== 'ready') {
      return '';
    }
    const name = s.creature.name;
    if (s.creature.source === CreatureSource.FAMILIAR && s.creature.attack === 1) {
      return `Como familiar, ${name} não ataca. Ele pode fazer as outras ações e entregar magias de toque (livro de regras).`;
    }
    if (s.creature.attack === 2) {
      return `${name} só ataca com a reação, quando alguém sai do alcance dele.`;
    }
    return '';
  });
  protected readonly canDismiss = computed(() => {
    const s = this.state();
    return s.status === 'ready' && (s.isMaster || s.creature.source !== CreatureSource.MASTER);
  });

  constructor() {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((p) => {
      this.campaignId.set(p.get('id') ?? '');
      this.characterId.set(p.get('characterId') ?? '');
      this.creatureId.set(p.get('creatureId') ?? '');
    });
    effect(() => {
      const campaignId = this.campaignId();
      const characterId = this.characterId();
      const creatureId = this.creatureId();
      if (campaignId && characterId && creatureId) {
        untracked(() => void this.load(campaignId, characterId, creatureId));
      }
    });
  }

  private async load(campaignId: string, characterId: string, creatureId: string): Promise<void> {
    try {
      const list = await this.client.list(campaignId, characterId);
      const creature = list.find((c) => c.id === creatureId);
      if (!creature) {
        this.state.set({ status: 'not-found' });
        return;
      }
      const [block, sheet] = await Promise.all([
        this.client.statBlock(campaignId, creature.monsterKey),
        this.sheets.getCharacterSheet(campaignId, characterId),
      ]);
      this.state.set({ status: 'ready', creature, block, ownerName: sheet.name, isMaster: sheet.isMaster });
    } catch (err) {
      const message = creatureErrorMessage(err, 'read');
      this.state.set(message.startsWith('Essa criatura não existe') ? { status: 'not-found' } : { status: 'error', message });
    }
  }

  protected open(mode: EditMode): void {
    this.mode.set(mode);
  }

  protected closed(changed: boolean): void {
    const was = this.mode();
    this.mode.set(null);
    if (!changed) {
      // Backing out: the focus goes back to the action that asked.
      afterNextRender(() => this.host.nativeElement.querySelector<HTMLElement>(`.js-${was}`)?.focus(), { injector: this.injector });
      return;
    }
    if (was === 'dismiss') {
      void this.router.navigate(this.back());
      return;
    }
    void this.load(this.campaignId(), this.characterId(), this.creatureId());
  }
}

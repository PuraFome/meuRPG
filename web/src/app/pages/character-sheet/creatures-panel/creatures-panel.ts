import { Component, computed, effect, inject, input, signal, untracked } from '@angular/core';
import { MatBottomSheet } from '@angular/material/bottom-sheet';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

import { type CharacterCreature, CreatureSource } from '../../../../gen/meurpg/characters/v1/characters_pb';
import type { CreatureAccessVm, SummonCastVm } from '../../../core/creatures/summon-access';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { creaturesHidden } from '../../../core/creatures/creature-errors';
import { tight } from '../../../core/format/text';
import { OpenSessions } from '../../../shell/live-notice/open-sessions';
import { openSheet } from '../../live-session/combat/sheet-host';
import { CreatureCard } from './creature-card';
import { SummonSheet, type SummonSheetData, type SummonSheetResult } from './summon-sheet';

type ListState = 'loading' | 'hidden' | 'ready';

/**
 * The sheet's "Criaturas" panel (E9-10, MR-037): the creatures that belong to
 * the character, one card each (see `CreatureCard`), and what the character can
 * conjure. It sits after "Combate" and "Magias", only for who can have a
 * creature: a character that casts Encontrar Familiar, Animar os Mortos or
 * Conjurar Animais, a druid, or any character the master gave one (no empty
 * panel for the rest). RN-20: the list is the owner's player's and the
 * master's; for anyone else the server answers `not_found` and the panel does
 * not exist.
 *
 * Empty: an invitation ("Nenhuma criatura ainda. Use Encontrar Familiar ou
 * peça ao mestre para dar uma.") and one outlined button per spell the sheet
 * can cast, with what it costs under it. A cast needs a session: outside one
 * the button is dashed and says why. The master does not cast from here (the
 * master gives, in the campaign's character list).
 *
 * A live region confirms what arrived ("Nanquim chegou. Encontrar Familiar,
 * ritual de 1 hora. Nenhum espaço de magia foi gasto.", "O mestre deu uma
 * criatura a você: Mastim."). The list is read again on the stream's
 * `creatures_changed` (`reload`, a counter the page bumps).
 */
@Component({
  selector: 'app-creatures-panel',
  imports: [CreatureCard, MatButtonModule, MatIconModule],
  templateUrl: './creatures-panel.html',
  styleUrl: './creatures-panel.scss',
})
export class CreaturesPanel {
  private readonly client = inject(CreaturesClient);
  private readonly dialog = inject(MatDialog);
  private readonly bottomSheet = inject(MatBottomSheet);
  private readonly openSessions = inject(OpenSessions);

  readonly campaignId = input.required<string>();
  readonly characterId = input.required<string>();
  readonly characterName = input.required<string>();
  readonly isMaster = input(false);
  readonly access = input.required<CreatureAccessVm>();
  /** Bumped by the page when the stream says the creatures changed. */
  readonly reload = input(0);

  protected readonly state = signal<ListState>('loading');
  protected readonly creatures = signal<readonly CharacterCreature[]>([]);
  /** What the live region says now. */
  protected readonly notice = signal('');
  private known = new Set<string>();
  private first = true;
  private castInFlight = false;

  protected readonly live = computed(() => this.openSessions.sessions().some((s) => s.campaignId === this.campaignId()));
  protected readonly casts = computed(() => (this.isMaster() ? [] : this.access().casts));
  protected readonly visible = computed(() => this.state() === 'ready' && (this.creatures().length > 0 || (!this.isMaster() && (this.casts().length > 0 || this.access().wildShape))));
  protected readonly countText = computed(() => {
    const n = this.creatures().length;
    return n === 0 ? '' : `${n} ${n === 1 ? 'criatura' : 'criaturas'}`;
  });
  protected readonly emptyText = computed(() => {
    const names = this.casts().map((c) => c.name);
    if (this.isMaster()) {
      return 'Nenhuma criatura ainda. Dê uma pela lista de personagens da campanha.';
    }
    if (names.length === 0) {
      return 'Nenhuma criatura ainda. Peça ao mestre para dar uma.';
    }
    return `Nenhuma criatura ainda. Use ${names.join(' ou ')} ou peça ao mestre para dar uma.`;
  });
  protected readonly familiar = computed(() => this.creatures().find((c) => c.source === CreatureSource.FAMILIAR) ?? null);

  constructor() {
    effect(() => {
      this.reload();
      const campaignId = this.campaignId();
      const characterId = this.characterId();
      untracked(() => void this.load(campaignId, characterId));
    });
  }

  private async load(campaignId: string, characterId: string): Promise<void> {
    try {
      const list = await this.client.list(campaignId, characterId);
      this.announceArrivals(list);
      this.creatures.set(list);
      this.state.set('ready');
    } catch (err) {
      // RN-20: a creature list nobody may read is no panel at all; any other failure keeps what is on screen.
      if (creaturesHidden(err)) {
        this.state.set('hidden');
      } else if (this.state() === 'loading') {
        this.state.set('hidden');
      }
    }
  }

  /** A creature that was not on the list before and did not come from a cast made here: the master gave it. */
  private announceArrivals(list: readonly CharacterCreature[]): void {
    const fresh = list.filter((c) => !this.known.has(c.id));
    this.known = new Set(list.map((c) => c.id));
    if (this.first) {
      this.first = false;
      return;
    }
    if (fresh.length === 0 || this.castInFlight) {
      return;
    }
    const gift = fresh.find((c) => c.source === CreatureSource.MASTER);
    this.notice.set(gift ? `O mestre deu uma criatura a você: ${gift.name}.` : `${fresh.map((c) => c.name).join(', ')} chegou.`);
  }

  protected costText(cast: SummonCastVm): string {
    const where = 'Só durante uma sessão, fora de combate.';
    if (cast.ritual) {
      return `Ritual de 1 hora: não gasta espaço de magia. ${where}`;
    }
    return tight(`Gasta um espaço de ${cast.level}º círculo ou maior. ${where}`);
  }

  protected cast(cast: SummonCastVm): void {
    if (!this.live()) {
      return;
    }
    const data: SummonSheetData = {
      campaignId: this.campaignId(),
      characterId: this.characterId(),
      cast,
      replaces: cast.key === 'spell:find-familiar' ? (this.familiar()?.name ?? '') : '',
    };
    this.castInFlight = true;
    openSheet<SummonSheet, SummonSheetData, SummonSheetResult>(this.dialog, this.bottomSheet, SummonSheet, {
      data,
      ariaLabel: cast.name,
      labelledBy: 'summon-t',
      width: '520px',
    }).subscribe((result) => {
      if (!result) {
        this.castInFlight = false;
        return;
      }
      void this.load(this.campaignId(), this.characterId()).then(() => {
        this.castInFlight = false;
        this.notice.set(this.confirmation(result));
      });
    });
  }

  private confirmation(r: SummonSheetResult): string {
    const who = r.names.length === 1 ? `${r.names[0]} chegou.` : 'As criaturas chegaram.';
    return r.ritual ? `${who} ${r.spellName}, ritual de 1 hora. Nenhum espaço de magia foi gasto.` : `${who} ${r.spellName}: o espaço de magia foi gasto.`;
  }

  protected refresh(): void {
    void this.load(this.campaignId(), this.characterId());
  }
}

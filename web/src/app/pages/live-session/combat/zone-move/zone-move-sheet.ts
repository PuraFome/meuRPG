import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import { CombatClient } from '../../../../core/combat/combat-client';
import type { Square } from '../../../../core/combat/combat-grid';
import type { CombatState } from '../../../../core/combat/combat-state';
import { zoneMoveError } from '../../../../core/combat/zone-errors';
import {
  ZONE_MOVE_RANGE_FT,
  zoneMovePreview,
  zoneRangeText,
} from '../../../../core/combat/zone-move';
import { article } from '../../../../core/format/article';
import { metersText } from '../../../../core/units';
import { ZoneMoveMap } from '../../../../shared/area-picker/zone-move-map';
import { injectSheet } from '../../../../shared/sheet/sheet-host';
import { SheetFrame } from '../../../../shared/sheet/sheet-frame/sheet-frame';
import type { CastMapData } from '../cast-sheet/cast-sheet';

/** What the page hands the move picker of a zone. */
export interface ZoneMoveSheetData {
  readonly campaignId: string;
  readonly encounterId: string;
  readonly zoneId: string;
  readonly master: boolean;
  readonly map: CastMapData;
  readonly state: CombatState;
}

/**
 * "Mover o Raio Lunar" (B6-04): the picker of where a zone that moves by an action goes, a bottom sheet on a phone and a
 * dialog from a tablet up. The map shows the current footprint dimmed and the new one; the words under it give the distance
 * and, for a player, whether it is in range. "Mover a zona" is the one filled button and stays off until a square other
 * than the current one, in range, is picked. The call carries one idempotency key per attempt (the client's keyed helper
 * renews it after a move worked); the sheet closes on success and says the server's refusal in place.
 */
@Component({
  selector: 'app-zone-move-sheet',
  imports: [MatButtonModule, MatIconModule, SheetFrame, ZoneMoveMap],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './zone-move-sheet.html',
  styleUrl: './zone-move-sheet.scss',
})
export class ZoneMoveSheet {
  private readonly api = inject(CombatClient);
  private readonly sheet = injectSheet<ZoneMoveSheetData, boolean>();
  protected readonly data = this.sheet.data;
  protected readonly inSheet = this.sheet.inSheet;

  protected readonly picked = signal<Square | null>(null);
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected readonly zone = computed(
    () => this.data.state.encounter()?.zones.find((z) => z.id === this.data.zoneId) ?? null,
  );
  protected readonly combatants = computed(() => this.data.state.encounter()?.combatants ?? []);
  protected readonly title = computed(() => {
    const name = this.zone()?.spellNamePt;
    return name ? `Mover ${article(name)} ${name}` : 'Mover a zona';
  });
  protected readonly subtitle = computed(() => {
    const caster = this.zone()?.ownerLabel ?? '';
    return this.data.master
      ? `Para onde? Sem limite de alcance e sem gastar a ação${caster ? ` de ${caster}` : ''}. As setas movem o quadrado.`
      : `Para onde? Até ${metersText(ZONE_MOVE_RANGE_FT)} do ponto atual. Gasta a ação${caster ? ` de ${caster}` : ''}. As setas movem o quadrado.`;
  });
  private readonly preview = computed(() => {
    const zone = this.zone();
    const at = this.picked();
    return zone && at ? zoneMovePreview(zone, at, this.data.master) : null;
  });
  protected readonly summary = computed(() => {
    const p = this.preview();
    return p
      ? { text: p.text, range: zoneRangeText(p, this.data.master), refused: !p.inRange }
      : null;
  });
  /** A square other than where the zone already is, and one the range allows. */
  protected readonly ready = computed(() => {
    const p = this.preview();
    return !!p && p.inRange && p.distanceFt > 0;
  });

  constructor() {
    effect(() => this.sheet.lock(this.busy()));
  }

  protected async move(): Promise<void> {
    const at = this.picked();
    if (this.busy() || !this.ready() || !at) {
      return;
    }
    this.busy.set(true);
    this.error.set('');
    try {
      this.data.state.apply(
        await this.api.casts.moveSpellZone(
          this.data.campaignId,
          this.data.encounterId,
          this.data.zoneId,
          at,
        ),
      );
      this.sheet.close(true);
    } catch (err) {
      this.error.set(zoneMoveError(err));
    } finally {
      this.busy.set(false);
    }
  }

  protected close(): void {
    if (!this.busy()) {
      this.sheet.close(false);
    }
  }
}

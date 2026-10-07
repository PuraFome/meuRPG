import { signal } from '@angular/core';

import type { StageNpc } from '../../../gen/meurpg/play/v1/scene_pb';
import type { SceneState } from './scene-state';
import { stageErrorMessage } from './scene-errors';
import { STAGE_LIMIT } from './stage-view';

/** The calls the controller makes: a slice of `SceneClient`, so a spec hands
 * it a fake. */
export interface StageApi {
  putOnStage(campaignId: string, characterId: string): Promise<readonly StageNpc[]>;
  takeOffStage(campaignId: string, characterId: string): Promise<readonly StageNpc[]>;
  setSpeaker(campaignId: string, characterId: string): Promise<readonly StageNpc[]>;
}

/**
 * The master's stage calls (MR-031), shared by the "Em cena" section and the
 * "Pôr em cena" sheet on a phone, which are two screens over one stage:
 *
 * - one write in flight per stage entry (`busy(characterId)`): a second tap on
 *   the same NPC does nothing until the first answers, while another NPC's
 *   buttons stay live;
 * - each answer is the stage as it is now, applied at once through
 *   `SceneState.setStage`;
 * - `status` is what the master's screen reader hears ("Aldo entrou na cena.")
 *   and `entered` is the same sentence for the visible green line after "Pôr
 *   em cena", until the next call;
 * - a refusal is mapped by code and typed detail (`stageErrorMessage`); a
 *   `failed_precondition` or `not_found` also reads the scene again, since the
 *   screen was stale.
 */
export class StageController {
  readonly status = signal('');
  readonly entered = signal('');
  readonly error = signal('');
  private readonly inFlight = signal<ReadonlySet<string>>(new Set());

  constructor(
    private readonly api: StageApi,
    private readonly state: SceneState,
    private readonly campaignId: () => string,
    /** The name of an NPC, for the sentences (the roster knows the ones that
     * are not on the stage yet). */
    private readonly nameOf: (characterId: string) => string,
  ) {}

  /** Whether a write for this NPC is on its way. */
  busy(characterId: string): boolean {
    return this.inFlight().has(characterId);
  }

  /** The stage has room for another NPC. */
  hasRoom(): boolean {
    return this.state.stage().length < STAGE_LIMIT;
  }

  /** "Pôr em cena": true when the NPC came in. */
  async put(characterId: string): Promise<boolean> {
    const ok = await this.run(characterId, () =>
      this.api.putOnStage(this.campaignId(), characterId),
    );
    if (ok) {
      const sentence = `${this.nameOf(characterId)} entrou na cena.`;
      this.entered.set(sentence);
      this.status.set(sentence);
    }
    return ok;
  }

  /** "Tirar de cena": at once, with no question (putting it back undoes it). */
  async take(characterId: string): Promise<boolean> {
    const name = this.nameOf(characterId);
    const ok = await this.run(characterId, () =>
      this.api.takeOffStage(this.campaignId(), characterId),
    );
    if (ok) {
      this.entered.set('');
      this.status.set(`${name} saiu da cena.`);
    }
    return ok;
  }

  /** "Dar a fala" / "Fala agora": the one who speaks, or, when the NPC
   * already speaks, nobody. */
  async speak(characterId: string, speaking: boolean): Promise<boolean> {
    const name = this.nameOf(characterId);
    const ok = await this.run(characterId, () =>
      this.api.setSpeaker(this.campaignId(), speaking ? '' : characterId),
    );
    if (ok) {
      this.entered.set('');
      this.status.set(speaking ? 'Ninguém fala.' : `${name} fala.`);
    }
    return ok;
  }

  clearMessages(): void {
    this.error.set('');
    this.entered.set('');
    this.status.set('');
  }

  private async run(
    characterId: string,
    call: () => Promise<readonly StageNpc[]>,
  ): Promise<boolean> {
    if (this.busy(characterId)) {
      return false;
    }
    this.inFlight.update((set) => new Set(set).add(characterId));
    this.error.set('');
    try {
      this.state.setStage(await call());
      return true;
    } catch (err) {
      this.error.set(stageErrorMessage(err));
      this.entered.set('');
      // The screen was stale (the scene closed, the NPC is gone): read it again.
      void this.state.refresh();
      return false;
    } finally {
      this.inFlight.update((set) => {
        const next = new Set(set);
        next.delete(characterId);
        return next;
      });
    }
  }
}

import { signal } from '@angular/core';

import type { GearClient, GearVm } from '../characters/gear';
import { combatErrorMessage } from './combat-errors';

/** What `GearFlow` reads from the screen it serves. */
export interface GearFlowContext {
  readonly client: Pick<GearClient, 'read' | 'set'>;
  readonly campaignId: () => string;
  /** The character of the screen's player; empty when the screen has none (the master's, a creature's). */
  readonly characterId: () => string;
}

/** The turn's economy as the gear rows need it. */
export interface GearTurn {
  readonly objectInteractionUsed: boolean;
  readonly master: boolean;
}

/** What a change of this piece costs now (SRD 5.1, "Other Activity on Your Turn" and "Armor"): the server charges it and
 * has the last word; the line only tells beforehand. Empty for the master, who is not charged. */
export function gearCost(piece: GearVm, turn: GearTurn): string {
  if (turn.master) {
    return 'Sem custo: o mestre muda o equipamento quando quiser.';
  }
  if (piece.kind === 'armor') {
    return '';
  }
  if (piece.kind === 'weapon' && !turn.objectInteractionUsed) {
    return 'Usa a interação gratuita do turno.';
  }
  return 'Usa a sua ação.';
}

/** Why a player cannot change this piece in a combat; empty when they can. */
export function gearBlocked(piece: GearVm, turn: GearTurn): string {
  return piece.kind === 'armor' && !turn.master
    ? 'Vestir ou tirar armadura leva minutos: não dá em pleno combate.'
    : '';
}

/**
 * The player's gear in a combat: the list the server derives, "Equipar" / "Guardar" and what the server answers. The
 * economy it spends (the free object interaction, the action) is the combat's: the stream brings it back, so the flow
 * only reads the gear again after a change.
 */
export class GearFlow {
  readonly gear = signal<readonly GearVm[]>([]);
  readonly armorClass = signal<number | null>(null);
  readonly busyKey = signal<string | null>(null);
  readonly notice = signal('');
  readonly error = signal('');
  private loadedFor = '';

  constructor(private readonly ctx: GearFlowContext) {}

  /** Reads the gear of the screen's character, once per character. */
  async load(): Promise<void> {
    const characterId = this.ctx.characterId();
    if (!characterId || characterId === this.loadedFor) {
      return;
    }
    this.loadedFor = characterId;
    await this.read(characterId);
  }

  private async read(characterId: string): Promise<boolean> {
    try {
      const sheet = await this.ctx.client.read(this.ctx.campaignId(), characterId);
      if (characterId !== this.ctx.characterId()) {
        return false;
      }
      this.gear.set(sheet.gear);
      this.armorClass.set(sheet.armorClass);
      return true;
    } catch {
      return false; // keeps what is shown: the next change reads again
    }
  }

  /** "Equipar" / "Guardar": the server's answer is the refusal in words, or the new list. `true` when it changed. */
  async toggle(key: string, equipped: boolean): Promise<boolean> {
    const characterId = this.ctx.characterId();
    if (!characterId || this.busyKey() !== null) {
      return false;
    }
    const name = this.gear().find((g) => g.key === key)?.namePt ?? '';
    this.busyKey.set(key);
    this.error.set('');
    this.notice.set('');
    try {
      await this.ctx.client.set(this.ctx.campaignId(), characterId, key, equipped);
      await this.read(characterId);
      const ac = this.armorClass();
      this.notice.set(
        `${equipped ? 'Equipado' : 'Guardado'}: ${name}.${ac === null ? '' : ` Classe de armadura ${ac}.`}`,
      );
      return true;
    } catch (err) {
      this.error.set(combatErrorMessage(err, 'mudar o equipamento'));
      return false;
    } finally {
      this.busyKey.set(null);
    }
  }
}

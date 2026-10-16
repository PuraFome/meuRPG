import { type DicePreference, XpMode } from '../../../gen/meurpg/campaigns/v1/campaigns_pb';
import {
  CharacterBlockedReason,
  type Character,
  type LevelUpOptions,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { LevelUpClient } from '../../core/levelup/levelup-client';
import { LevelUpDraft } from '../../core/levelup/levelup-draft';
import {
  cannotLevelUpMessage,
  describeLevelUpFailure,
  type LevelUpFailure,
} from '../../core/levelup/levelup-errors';
import type { SheetKeys } from '../../core/levelup/levelup-flow';
import { LevelUpSession } from './level-up-session';

export type PageState =
  | { readonly status: 'loading' }
  /** Nothing to level up here: not the player's, not allowed yet, dead, or not found. */
  | { readonly status: 'blocked'; readonly message: string }
  | { readonly status: 'error'; readonly message: string }
  | { readonly status: 'ready'; readonly session: LevelUpSession };

/** The keys the sheet already has, for the pickers (a content key is never offered twice). */
export function sheetKeys(character: Character): SheetKeys {
  const full = character.sheet?.content.case === 'full' ? character.sheet.content.value : null;
  return {
    cantrips: full?.cantripKeys ?? [],
    known: full?.knownSpellKeys ?? [],
    prepared: full?.preparedSpellKeys ?? [],
    skills: full?.skillProficiencyKeys ?? [],
    expertise: full?.expertiseSkillKeys ?? [],
  };
}

/** How many choices the person has made, to tell whether a re-read dropped one. */
export function pickedCount(d: LevelUpDraft): number {
  return (
    (d.subclassKey() ? 1 : 0) +
    d.cantrips().size +
    d.spells().size +
    d.prepared().size +
    d.features().size +
    d.skills().size +
    d.expertise().size
  );
}

/** What the page offers to choose from (the subclasses, the spells, the classes): a change in it is what the note is about. */
export function offerSignature(s: LevelUpSession): string {
  const c = s.draft.catalog;
  return JSON.stringify([
    s.options.subclasses.map((k) => [k.key, k.off, k.archived]),
    c.spells.map((x) => x.key),
    (c.classes ?? []).map((x) => x.key),
  ]);
}

/** The words for a master who opens the player's page. */
const MASTER_BLOCKED =
  'Quem sobe o nível é o jogador, pelo botão da ficha. O mestre ajusta a ficha pelo editor.';

/** What the reader needs of the page it reads for: where the person is, and what to show. */
export interface ReaderView {
  readonly campaignId: () => string;
  readonly characterId: () => string;
  readonly session: () => LevelUpSession | null;
  readonly setState: (state: PageState) => void;
  readonly clearFailure: () => void;
  readonly show: (failure: LevelUpFailure) => void;
  readonly describeSpell: (key: string, name: string) => void;
  /** A session was made from the first reading, and is on screen. */
  readonly ready: () => void;
  /** The class step: the options of another class are being read, and what came of it. */
  readonly classBusy: (busy: boolean) => void;
  readonly classSwitched: (session: LevelUpSession) => void;
  readonly classFailed: (failure: LevelUpFailure) => void;
}

/**
 * Everything the level-up page reads from the server to make a `LevelUpSession`: the first reading, the re-reads after a stale
 * revision or a content change, and the reading of another class's options. It numbers its reads, so of two that overlap only
 * the newest lands. The page keeps what is on screen; this makes the sessions it shows.
 */
export class LevelUpReader {
  /** Numbers every read that makes a session (the first one and each re-read): an answer older than the latest is dropped,
   * so the reading on screen is always the newest one asked for. */
  private reads = 0;
  private classSwitches = 0;

  constructor(
    private readonly client: LevelUpClient,
    private readonly view: ReaderView,
  ) {}

  private session(
    character: Character,
    options: LevelUpOptions,
    draft: LevelUpDraft,
    preference: DicePreference,
  ): LevelUpSession {
    return new LevelUpSession(
      this.view.campaignId(),
      character,
      options,
      draft,
      this.client,
      preference,
      (key, name) => this.view.describeSpell(key, name),
    );
  }

  async load(campaignId: string, characterId: string): Promise<void> {
    const seq = ++this.reads;
    this.view.setState({ status: 'loading' });
    let character: Character | null = null;
    try {
      character = await this.client.character(campaignId, characterId);
      if (seq !== this.reads) {
        return;
      }
      if (character.canAccessMasterNotes) {
        // The master's own call: the owning player levels up; the master edits the sheet.
        this.view.setState({ status: 'blocked', message: MASTER_BLOCKED });
        return;
      }
      const [options, catalog, preference] = await Promise.all([
        this.client.options(campaignId, characterId),
        this.client.catalog(campaignId, characterId),
        this.client.dicePreference(campaignId).catch(() => 0),
      ]);
      if (seq !== this.reads) {
        return;
      }
      const draft = new LevelUpDraft(options, sheetKeys(character), catalog);
      this.view.setState({
        status: 'ready',
        session: this.session(character, options, draft, preference),
      });
      this.view.ready();
    } catch (err) {
      if (seq !== this.reads) {
        return;
      }
      const failure = describeLevelUpFailure(err);
      const message = await this.refusalMessage(campaignId, character, failure);
      if (seq !== this.reads) {
        return;
      }
      this.view.setState({ status: failure.kind === 'blocked' ? 'blocked' : 'error', message });
    }
  }

  /** The campaign's mode and the sheet's XP are known: say what is missing, not only that something is. */
  private async refusalMessage(
    campaignId: string,
    character: Character | null,
    failure: LevelUpFailure,
  ): Promise<string> {
    if (
      failure.kind !== 'blocked' ||
      failure.reason !== CharacterBlockedReason.CANNOT_LEVEL_UP ||
      !character
    ) {
      return failure.message;
    }
    const mode = await this.client.xpMode(campaignId).catch(() => XpMode.UNSPECIFIED);
    const full = character.sheet?.content.case === 'full' ? character.sheet.content.value : null;
    return cannotLevelUpMessage(
      mode,
      full?.experiencePoints ?? 0,
      character.derived?.nextLevelXp ?? 0,
      character.derived?.totalLevel ?? 0,
    );
  }

  /** After a stale revision or a content change: the sheet, what the level gives and the lists (read fresh, never from the
   * client's memory) are read again, the session is made anew, and the picks that are still offered are carried over.
   * Resolves true when the page now shows the new reading. */
  async reread(afterContent = false): Promise<boolean> {
    const old = this.view.session();
    if (!old) {
      return false;
    }
    const seq = ++this.reads;
    const campaignId = this.view.campaignId();
    const characterId = this.view.characterId();
    try {
      // The table's content is read again too: the master may have retired an option, or written a new one.
      const [character, options, catalog] = await Promise.all([
        this.client.character(campaignId, characterId),
        this.client.options(campaignId, characterId, old.options.classKey),
        this.client.catalog(campaignId, characterId, true),
      ]);
      if (seq !== this.reads) {
        return false; // a newer reading was asked for: its answer is the one that counts
      }
      const draft = new LevelUpDraft(options, sheetKeys(character), catalog);
      draft.adopt(old.draft);
      const session = this.session(character, options, draft, old.preference);
      // A roll answered after this point lands on the new session (the click was made on the old one).
      old.handOver(session);
      this.view.clearFailure();
      this.view.setState({ status: 'ready', session });
      return true;
    } catch (err) {
      if (seq !== this.reads) {
        return false;
      }
      const failure = describeLevelUpFailure(err);
      if (failure.kind === 'blocked') {
        old.stop();
        this.view.setState({ status: 'blocked', message: failure.message });
      } else if (!afterContent) {
        this.view.show(failure);
      }
      return false;
    }
  }

  /** `content_changed` (RN-23): the options and the lists are read again with this person's role; the choices that are still
   * offered stay (`adopt`). Resolves to what the page says about it: that the lists changed, or that a choice left them. It is
   * empty while what the page shows (the offers and the lists) is the same. */
  async contentChanged(): Promise<string> {
    const old = this.view.session();
    if (!old) {
      return '';
    }
    const before = pickedCount(old.draft);
    const offers = offerSignature(old);
    const changed = await this.reread(true);
    const now = this.view.session();
    if (!changed || !now) {
      return '';
    }
    if (pickedCount(now.draft) < before) {
      return 'O mestre mudou as opções da mesa e uma das suas escolhas saiu da lista. Escolha de novo antes de confirmar.';
    }
    return offerSignature(now) !== offers
      ? 'O mestre mudou as opções da mesa. As listas deste nível estão atualizadas.'
      : '';
  }

  /** "Trocar para o ...": the options of another class are read and a session is made for it, with no choices. */
  async switchClass(key: string): Promise<void> {
    const old = this.view.session();
    if (!old) {
      return;
    }
    const turn = ++this.classSwitches;
    this.view.classBusy(true);
    this.view.clearFailure();
    try {
      const options = await this.client.options(
        this.view.campaignId(),
        this.view.characterId(),
        key,
      );
      if (turn !== this.classSwitches) {
        return;
      }
      const draft = new LevelUpDraft(options, sheetKeys(old.character), old.draft.catalog);
      // A die rolled on its way for the old class lands on the old session, which is off screen: never on this class.
      const session = this.session(old.character, options, draft, old.preference);
      old.stop();
      this.view.classSwitched(session);
    } catch (err) {
      if (turn === this.classSwitches) {
        this.view.classFailed(describeLevelUpFailure(err));
      }
    } finally {
      if (turn === this.classSwitches) {
        this.view.classBusy(false);
      }
    }
  }
}

import {
  type AttunementEntryVm,
  type AttunementVm,
  MAX_ATTUNED,
} from '../../core/characters/attunement';

/** Who looks at the section: the owner (inside the window a rest opens) or the master (any time, past a restriction). */
export type AttunementViewer = 'owner' | 'master';

/** One row of the section. `button` is `on` (a tap asks the server), `dashed` (the server would refuse: the button is
 * `aria-disabled` and the reason sits under the row) or `none` (the owner has no button now). */
export interface AttunementRowVm {
  readonly entry: AttunementEntryVm;
  readonly verb: 'Sintonizar' | 'Encerrar';
  readonly button: 'on' | 'dashed' | 'none';
  /** The reason under the row, empty without one. */
  readonly reason: string;
  readonly reasonIcon: 'block' | 'info' | 'dark_mode' | '';
}

export interface AttunementSectionVm {
  readonly title: string;
  readonly count: string;
  readonly lead: string;
  /** "Limite de 3 itens. ..." above the rows that ask, only while the limit is reached and there are some. */
  readonly limitLine: string;
  readonly asking: readonly AttunementRowVm[];
  readonly attuned: readonly AttunementRowVm[];
  /** "Nada nesta ficha pede sintonização." */
  readonly empty: string;
}

export interface AttunementContext {
  readonly viewer: AttunementViewer;
  /** The character's name, for what the master reads ("Tor não cumpre"). */
  readonly name: string;
  /** The window a rest opened: it matters for the owner only. */
  readonly open: boolean;
  readonly dead: boolean;
}

const CIRCUMSTANCE = /noite|ar livre/i;

function reasonOf(
  e: AttunementEntryVm,
  c: AttunementContext,
): Pick<AttunementRowVm, 'reason' | 'reasonIcon'> {
  if (e.sameItemAttuned) {
    return { reason: 'Já sintonizado a outro igual.', reasonIcon: 'block' };
  }
  const by = e.restrictionPt;
  if (by === '') {
    return { reason: '', reasonIcon: '' };
  }
  if (e.restriction === 'notMet') {
    const who = c.viewer === 'owner' ? 'você não cumpre' : `${c.name} não cumpre`;
    return {
      reason:
        c.viewer === 'owner'
          ? `${by}: ${who}.`
          : `${by}: ${who}. Como mestre, você pode sintonizar mesmo assim.`,
      reasonIcon: c.viewer === 'owner' ? 'block' : 'info',
    };
  }
  if (e.restriction === 'unknown') {
    return c.viewer === 'owner'
      ? {
          reason: `${by}: a ficha não diz o alinhamento. Só o mestre pode sintonizar.`,
          reasonIcon: 'block',
        }
      : {
          reason: `${by}: a ficha não diz o alinhamento de ${c.name}. Você decide.`,
          reasonIcon: 'info',
        };
  }
  return { reason: `${by}.`, reasonIcon: CIRCUMSTANCE.test(by) ? 'dark_mode' : 'info' };
}

function askingRow(e: AttunementEntryVm, c: AttunementContext, atLimit: boolean): AttunementRowVm {
  const why = reasonOf(e, c);
  let button: AttunementRowVm['button'] = 'on';
  if (c.viewer === 'owner' && (c.dead || !c.open)) {
    button = 'none';
  } else if (atLimit || e.sameItemAttuned) {
    button = 'dashed';
  } else if (c.viewer === 'owner' && e.restriction !== 'none' && e.restriction !== 'met') {
    button = 'dashed';
  }
  return { entry: e, verb: 'Sintonizar', button, ...why };
}

function attunedRow(e: AttunementEntryVm, c: AttunementContext): AttunementRowVm {
  const none = c.viewer === 'owner' && (c.dead || !c.open);
  return { entry: e, verb: 'Encerrar', button: none ? 'none' : 'on', reason: '', reasonIcon: '' };
}

function leadOf(c: AttunementContext, atLimit: boolean): string {
  if (c.viewer === 'master') {
    return c.dead
      ? 'Morto: as sintonizações ficam como estavam até você encerrar.'
      : 'Você é o mestre: sintoniza e encerra a qualquer hora, sem descanso.';
  }
  if (c.dead) {
    return 'Morto: os itens e as sintonizações ficam como estavam.';
  }
  if (!c.open) {
    return 'A sintonização deste descanso foi usada. A próxima abre no próximo descanso, curto ou longo.';
  }
  return atLimit
    ? 'Neste descanso você pode encerrar um item.'
    : 'Neste descanso você pode sintonizar ou encerrar um item.';
}

/** The section for a sheet: the rows, the buttons and the sentences the boards draw. The server decides who meets what
 * (`restriction`); this only words it and picks which button is on. */
export function sectionOf(a: AttunementVm, c: AttunementContext): AttunementSectionVm {
  const atLimit = a.attuned >= MAX_ATTUNED;
  const asking =
    c.viewer === 'master' && c.dead
      ? []
      : a.entries.filter((e) => e.asks && !e.attuned).map((e) => askingRow(e, c, atLimit));
  const attuned = a.entries.filter((e) => e.attuned).map((e) => attunedRow(e, c));
  const limitLine =
    atLimit && asking.length > 0 && !(c.viewer === 'owner' && (c.dead || !c.open))
      ? c.viewer === 'owner'
        ? 'Limite de 3 itens. Encerre uma agora; sintonizar outro fica para o próximo descanso.'
        : 'Limite de 3 itens. Encerre um antes de sintonizar outro.'
      : '';
  return {
    title: c.viewer === 'master' ? `Sintonização · ${c.name}` : 'Sintonização',
    count: `Sintonizados: ${a.attuned} de ${MAX_ATTUNED}`,
    lead: leadOf(c, atLimit),
    limitLine,
    asking,
    attuned,
    empty: asking.length === 0 && attuned.length === 0 ? 'Nada nesta ficha pede sintonização.' : '',
  };
}

import { create } from '@bufbuild/protobuf';
import { type Timestamp, timestampDate } from '@bufbuild/protobuf/wkt';

import {
  type CampaignExport,
  type PackageCounts,
  PackageCountsSchema,
  type PreviewCampaignImportResponse,
} from '../../../gen/meurpg/campaignpackage/v1/campaignpackage_pb';
import { formatBytes, joinNames } from '../images/image-format';
import type { UploadFailureKind } from '../images/upload-errors';
import { MAX_PACKAGE_BYTES } from './package-problems';

const KIB = 1024;
const MIB = KIB * KIB;
const HOURS_PER_DAY = 24;
const MINUTES_PER_HOUR = 60;
const SECONDS_PER_MINUTE = 60;
const MS_PER_SECOND = 1000;
const MS_PER_DAY = HOURS_PER_DAY * MINUTES_PER_HOUR * SECONDS_PER_MINUTE * MS_PER_SECOND;
/** The server takes a file name of 1 to 120 characters and a fingerprint of 1 to 200 (campaignpackage.proto). */
const FILE_NAME_MAX = 120;
const FINGERPRINT_MAX = 200;
/** How many problems the refused card lists before it says "E mais N". */
export const PROBLEMS_SHOWN = 20;

/** A count with its noun: "1 mapa", "6 mapas". */
export function counted(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

// ---------------------------------------------------------------------------------------------------------
// Importing: the file the person chose

export type FileRefusal = 'EMPTY' | 'TOO_BIG' | 'NOT_ZIP';

/** The fast, client-side half of the checks: the server stays the authority (it reads the bytes). */
export function precheckPackageFile(file: Pick<File, 'name' | 'size'>): FileRefusal | null {
  if (!file.name.toLowerCase().endsWith('.zip')) {
    return 'NOT_ZIP';
  }
  if (file.size <= 0) {
    return 'EMPTY';
  }
  return file.size > MAX_PACKAGE_BYTES ? 'TOO_BIG' : null;
}

export function fileRefusalText(refusal: FileRefusal): string {
  switch (refusal) {
    case 'NOT_ZIP':
      return 'Esse arquivo não parece um pacote de campanha. Ele precisa terminar em .meurpg.zip (ou .zip).';
    case 'EMPTY':
      return 'Esse arquivo está vazio. Escolha o pacote .meurpg.zip que você exportou.';
    case 'TOO_BIG':
      return `Esse arquivo passa de ${formatBytes(MAX_PACKAGE_BYTES)}, o limite de um pacote.`;
  }
}

/**
 * What makes "the same file again" the same: its name, size and last-modified time, so choosing it again
 * resumes the upload the server kept (`BeginCampaignImport`). The size and time come first, so a long name
 * cut to fit the server's limit never hides them.
 */
export function fingerprintOf(file: Pick<File, 'name' | 'size' | 'lastModified'>): string {
  return `${file.size}:${file.lastModified}:${file.name}`.slice(0, FINGERPRINT_MAX);
}

/** The name `BeginCampaignImport` takes, which is only shown back to the person. */
export function beginName(file: Pick<File, 'name'>): string {
  return file.name.slice(0, FILE_NAME_MAX);
}

/** "Enviando em partes: parte 7 de 17 (41%). …": the text of the live region, which changes once per part. */
export function uploadProgressText(part: number, partCount: number): string {
  const percent = Math.floor((100 * part) / Math.max(partCount, 1));
  return `Enviando em partes: parte ${part} de ${partCount} (${percent}%). Se a conexão cair, continua de onde parou. As partes ficam guardadas por 1 hora.`;
}

/** The sentence under a failed upload, after "O envio não terminou." */
export function importUploadFailureText(kind: UploadFailureKind): string {
  switch (kind) {
    case 'NETWORK':
      return 'A conexão caiu durante o envio.';
    case 'UNAVAILABLE':
      return 'O servidor não está recebendo o envio agora.';
    case 'RATE_LIMITED':
      return 'Muitos envios em pouco tempo.';
    case 'UNAUTHENTICATED':
      return 'Sua sessão acabou. Entre de novo para continuar.';
    case 'PERMISSION_DENIED':
      return 'Este servidor não deixa você importar campanhas.';
    case 'NOT_FOUND':
      return 'O envio não existe mais: as partes ficam guardadas só por 1 hora.';
    case 'TOO_LARGE':
    case 'QUOTA':
      return `Esse arquivo passa de ${formatBytes(MAX_PACKAGE_BYTES)}, o limite de um pacote.`;
    case 'CANCELED':
      return 'Envio cancelado.';
    default:
      return 'Algo deu errado no envio.';
  }
}

/** The card of an upload the server still holds (`GetCampaignImport`): what it is and how to carry on. */
export function keptUploadText(upload: {
  fileName: string;
  totalBytes: bigint;
  partCount: number;
  receivedParts: readonly number[];
}): string {
  const held = `${upload.receivedParts.length} de ${upload.partCount} ${upload.partCount === 1 ? 'parte' : 'partes'}`;
  return `${upload.fileName} (${formatBytes(Number(upload.totalBytes))}): ${held} já foram enviadas e ficam guardadas por pouco tempo. Escolha o mesmo arquivo de novo e o envio continua de onde parou.`;
}

// ---------------------------------------------------------------------------------------------------------
// Importing: the preview and the campaign made

export interface CountRow {
  readonly label: string;
  readonly value: string;
}

/** The preview's grid, in the board's order. A missing `counts` reads as all zeros. */
export function countRows(counts: PackageCounts | undefined): readonly CountRow[] {
  const c = counts ?? create(PackageCountsSchema);
  return [
    { label: 'Mapas', value: String(c.maps) },
    { label: 'NPCs e criaturas', value: String(c.npcs) },
    { label: 'Cenas', value: String(c.scenes) },
    { label: 'Quebra-cabeças', value: String(c.puzzles) },
    { label: 'Pontos de batalha e encontros', value: String(c.battlePoints) },
    { label: 'Pontos de tesouro', value: String(c.treasurePoints) },
    { label: 'Imagens', value: `${c.images} (${formatBytes(Number(c.imageBytes))})` },
    { label: 'Conteúdo da mesa', value: counted(c.contentEntries, 'entrada', 'entradas') },
    { label: 'Personagens (reservados)', value: String(c.characters) },
  ];
}

/** "08/10": the day and month, in the browser's time zone. */
export function dayMonth(date: Date): string {
  return date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit' });
}

/** "Mirathel · exportado em 08/10 · formato do pacote 1": each part only when the package said it. */
export function previewHeadline(
  preview: Pick<PreviewCampaignImportResponse, 'campaignName' | 'exportedAt' | 'formatVersion'>,
): string {
  const parts: string[] = [];
  if (preview.campaignName !== '') {
    parts.push(preview.campaignName);
  }
  if (preview.exportedAt) {
    parts.push(`exportado em ${dayMonth(timestampDate(preview.exportedAt))}`);
  }
  if (preview.formatVersion > 0) {
    parts.push(`formato do pacote ${preview.formatVersion}`);
  }
  return parts.join(' · ');
}

/**
 * What the new campaign came with, after its name: "6 mapas, 24 NPCs e criaturas e 4 personagens reservados."
 * Only what the package had (a zero is left out); the name is the screen's to bold. A package with none of
 * these gives just "." — see `createdSentenceTail`.
 */
export function createdItems(counts: PackageCounts | undefined): string[] {
  const items: string[] = [];
  const maps = counts?.maps ?? 0;
  const npcs = counts?.npcs ?? 0;
  const characters = counts?.characters ?? 0;
  if (maps > 0) {
    items.push(counted(maps, 'mapa', 'mapas'));
  }
  if (npcs > 0) {
    items.push(counted(npcs, 'NPC ou criatura', 'NPCs e criaturas'));
  }
  if (characters > 0) {
    items.push(counted(characters, 'personagem reservado', 'personagens reservados'));
  }
  return items;
}

/** The sentence after the campaign's name: " foi criada com 6 mapas, … e 4 personagens reservados." or " foi criada." */
export function createdSentenceTail(counts: PackageCounts | undefined): string {
  const items = createdItems(counts);
  return items.length === 0 ? ' foi criada.' : ` foi criada com ${joinNames(items)}.`;
}

/** "E mais 3 problemas." under the list the refused card cut. */
export function moreProblemsText(hidden: number): string {
  return `E ${hidden === 1 ? 'mais 1 problema' : `mais ${hidden} problemas`}.`;
}

// ---------------------------------------------------------------------------------------------------------
// Exporting

/** The campaign's part of the file name stops here, as on the server (`maxFileNameLetters`). */
const MAX_FILE_NAME_LETTERS = 60;

/**
 * `<campanha>.meurpg.zip` exactly as the server names the download: a port of `campaignpackage.FileName`, so the
 * page shows the name the file will have ("Mirathel verificação" is "Mirathel_verificacao.meurpg.zip"). Accents come
 * off; anything but ASCII letters, digits, dots, dashes and underscores becomes one underscore; nothing is left at the
 * edges; a name with nothing usable is "campanha". Keep it in step with `format.go` (`TestFileNamesAreSafe`).
 */
export function suggestedFileName(campaignName: string): string {
  let name = '';
  let lastUnderscore = true; // no underscore at the start
  for (const ch of campaignName.normalize('NFD')) {
    if (/\p{Mn}/u.test(ch)) {
      continue; // the accent the decomposition split off
    }
    if (/^[A-Za-z0-9]$/.test(ch)) {
      name += ch;
      lastUnderscore = false;
    } else if (ch === '-' || ch === '_' || ch === '.') {
      if (!lastUnderscore || ch !== '.') {
        name += ch;
      }
      lastUnderscore = ch === '_';
    } else if (!lastUnderscore) {
      name += '_';
      lastUnderscore = true;
    }
    if (name.length >= MAX_FILE_NAME_LETTERS) {
      break;
    }
  }
  const safe = name.replace(/^[._-]+|[._-]+$/g, '');
  return `${safe === '' ? 'campanha' : safe}.meurpg.zip`;
}

/** "cerca de 84 MB": a whole number, never "cerca de 0 MB". */
export function approxSize(bytes: number): string {
  return `cerca de ${Math.max(1, Math.round(bytes / MIB))} MB`;
}

export type DayWord = 'hoje' | 'amanhã' | 'ontem' | { readonly date: string };

function dayNumber(date: Date): number {
  return Math.round(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()) / MS_PER_DAY);
}

/** "hoje", "amanhã" or "ontem" by the calendar days in the browser's time zone; any other day by its date. */
export function dayWord(date: Date, now: Date): DayWord {
  switch (dayNumber(date) - dayNumber(now)) {
    case 0:
      return 'hoje';
    case 1:
      return 'amanhã';
    case -1:
      return 'ontem';
    default:
      return { date: dayMonth(date) };
  }
}

/** "19:05" */
export function clockTime(date: Date): string {
  return date.toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });
}

/** "amanhã às 19:05", or "12/10 às 19:05". `at` is the preposition for a date only ("em 12/10 às 19:05"). */
function dayAndTime(date: Date, now: Date, dateLead: string): string {
  const day = dayWord(date, now);
  const word = typeof day === 'string' ? day : `${dateLead}${day.date}`;
  return `${word} às ${clockTime(date)}`;
}

/** "O arquivo fica guardado por 24 horas (até amanhã às 19:05) e depois é apagado. Só o mestre baixa." */
export function expiryText(expiresAt: Timestamp | undefined, now: Date = new Date()): string {
  const until = expiresAt ? ` (até ${dayAndTime(timestampDate(expiresAt), now, '')})` : '';
  return `O arquivo fica guardado por 24 horas${until} e depois é apagado. Só o mestre baixa.`;
}

/** "84,2 MB · 312 itens · gerado hoje às 19:05." (the screen puts the file name, in bold, before it) */
export function lastPackageText(
  exp: Pick<CampaignExport, 'byteSize' | 'entryCount' | 'finishedAt'>,
  now: Date = new Date(),
): string {
  const parts = [formatBytes(Number(exp.byteSize)), counted(exp.entryCount, 'item', 'itens')];
  const when = exp.finishedAt
    ? ` · gerado ${dayAndTime(timestampDate(exp.finishedAt), now, 'em ')}`
    : '';
  return `${parts.join(' · ')}${when}.`;
}

/** Where a running export is, in words (the percent is the server's; the words are the app's). */
const PHASES: readonly { readonly from: number; readonly text: string }[] = [
  { from: 0, text: 'Lendo a campanha' },
  { from: 20, text: 'Juntando os mapas e as imagens' },
  { from: 80, text: 'Fechando o arquivo' },
  { from: 100, text: 'Guardando o arquivo' },
];

function clampPercent(percent: number): number {
  return Math.min(100, Math.max(0, Math.round(percent)));
}

/** "Juntando os mapas e as imagens": where a running export is, in words. */
export function exportPhase(percent: number): string {
  const p = clampPercent(percent);
  return ([...PHASES].reverse().find((f) => p >= f.from) ?? PHASES[0]).text;
}

/** "Juntando os mapas e as imagens (62%). Pode fechar esta página: o arquivo fica pronto por 24 horas." */
export function exportProgressText(percent: number): string {
  return `${exportPhase(percent)} (${clampPercent(percent)}%). ${EXPORT_CLOSE_NOTE}`;
}

export const EXPORT_CLOSE_NOTE = 'Pode fechar esta página: o arquivo fica pronto por 24 horas.';

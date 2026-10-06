import { Component } from '@angular/core';

/**
 * The exact SRD 5.1 attribution text CC-BY-4.0 requires — copied, once, from
 * the "Legal Information" section of the SRD 5.1 PDF, as recorded in the
 * private ADR-0008 (`docs/adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md`,
 * not in this worktree). Do not edit this string without checking that ADR:
 * the license requires it verbatim, and `creditos.spec.ts` checks it
 * byte-for-byte. The backend's `NOTICE` file and `srd51.Attribution` (owned
 * by the `rules` module) carry the same text independently.
 */
export const SRD_ATTRIBUTION =
  'This work includes material taken from the System Reference Document 5.1 ("SRD 5.1") by Wizards of the Coast LLC and available at https://dnd.wizards.com/resources/systems-reference-document. The SRD 5.1 is licensed under the Creative Commons Attribution 4.0 International License available at https://creativecommons.org/licenses/by/4.0/legalcode.';

/**
 * The exact SRD 5.2.1 attribution (CC-BY-4.0), as in the repository's `NOTICE`: three tables of the
 * 2024 rules (the ways of making ability scores, the encounter XP budget, the magic item values) are
 * used and labelled "SRD 5.2.1 (regras de 2024)" wherever they show.
 */
export const SRD_521_ATTRIBUTION =
  'This work includes material from the System Reference Document 5.2.1 (“SRD 5.2.1”) by Wizards of the Coast LLC, available at https://www.dndbeyond.com/srd. The SRD 5.2.1 is licensed under the Creative Commons Attribution 4.0 International License, available at https://creativecommons.org/licenses/by/4.0/legalcode.';

/**
 * "/creditos": public (not behind `authGuard`), linked from the footer on
 * every page. Required by CC-BY-4.0 for the SRD 5.1 content the `rules`
 * module ships. The SRD 5.1 PDF's own terms ask that no other attribution to
 * Wizards of the Coast be added, and allow saying the work is "compatible
 * with fifth edition" — this page does exactly that and never uses the
 * "D&D" or "Dungeons & Dragons" trademark, and neither does any other screen
 * in this app.
 */
@Component({
  selector: 'app-creditos',
  templateUrl: './creditos.html',
  styleUrl: './creditos.scss',
})
export class Creditos {
  protected readonly srdAttribution = SRD_ATTRIBUTION;
  protected readonly srd521Attribution = SRD_521_ATTRIBUTION;
}

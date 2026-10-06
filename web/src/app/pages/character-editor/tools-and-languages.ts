import type { ToolOrLanguageVm } from './character-editor.types';

/**
 * The SRD's tools and languages an "Outro" background may grant (SRD 5.1 "Customizing a
 * Background": two tools or languages, in any mix). The keys are the SRD's own
 * (`proficiency:*` of kind tool or other, and `language:*`), the ones the server accepts in
 * `CustomBackground.proficiency_keys`; the names are ours, in Portuguese.
 *
 * `ContentService.ListContent` has no list of them yet, so this is the one catalog the editor
 * keeps by hand. It is only a pick-list of keys: the server still checks every key, and the sheet's
 * numbers still come from it. Ask for them in the catalog when a slice next touches `Content`.
 */
const TOOLS: readonly [string, string][] = [
  ['alchemists-supplies', 'Suprimentos de alquimista'],
  ['bagpipes', 'Gaita de foles'],
  ['brewers-supplies', 'Suprimentos de cervejeiro'],
  ['calligraphers-supplies', 'Suprimentos de calígrafo'],
  ['carpenters-tools', 'Ferramentas de carpinteiro'],
  ['cartographers-tools', 'Ferramentas de cartógrafo'],
  ['cobblers-tools', 'Ferramentas de sapateiro'],
  ['cooks-utensils', 'Utensílios de cozinheiro'],
  ['dice-set', 'Conjunto de dados'],
  ['disguise-kit', 'Kit de disfarce'],
  ['drum', 'Tambor'],
  ['dulcimer', 'Saltério'],
  ['flute', 'Flauta'],
  ['forgery-kit', 'Kit de falsificação'],
  ['glassblowers-tools', 'Ferramentas de vidreiro'],
  ['herbalism-kit', 'Kit de herbalismo'],
  ['horn', 'Trompa'],
  ['jewelers-tools', 'Ferramentas de joalheiro'],
  ['land-vehicles', 'Veículos terrestres'],
  ['leatherworkers-tools', 'Ferramentas de curtidor'],
  ['lute', 'Alaúde'],
  ['lyre', 'Lira'],
  ['masons-tools', 'Ferramentas de pedreiro'],
  ['navigators-tools', 'Ferramentas de navegador'],
  ['painters-supplies', 'Suprimentos de pintor'],
  ['pan-flute', 'Flauta de pã'],
  ['playing-card-set', 'Baralho'],
  ['poisoners-kit', 'Kit de envenenador'],
  ['potters-tools', 'Ferramentas de oleiro'],
  ['shawm', 'Charamela'],
  ['smiths-tools', 'Ferramentas de ferreiro'],
  ['thieves-tools', 'Ferramentas de ladrão'],
  ['tinkers-tools', 'Ferramentas de funileiro'],
  ['viol', 'Viola'],
  ['water-vehicles', 'Veículos aquáticos'],
  ['weavers-tools', 'Ferramentas de tecelão'],
  ['woodcarvers-tools', 'Ferramentas de entalhador'],
];

const LANGUAGES: readonly [string, string][] = [
  ['abyssal', 'Abissal'],
  ['celestial', 'Celestial'],
  ['common', 'Comum'],
  ['deep-speech', 'Dialeto das Profundezas'],
  ['draconic', 'Dracônico'],
  ['dwarvish', 'Anão'],
  ['elvish', 'Élfico'],
  ['giant', 'Gigante'],
  ['gnomish', 'Gnômico'],
  ['goblin', 'Goblin'],
  ['halfling', 'Halfling'],
  ['infernal', 'Infernal'],
  ['orc', 'Orc'],
  ['primordial', 'Primordial'],
  ['sylvan', 'Silvestre'],
  ['undercommon', 'Subcomum'],
];

const COLLATOR = new Intl.Collator('pt-BR');

/** Tools first, then languages, each in alphabetical order of its Portuguese name. */
export const TOOLS_AND_LANGUAGES: readonly ToolOrLanguageVm[] = [
  ...TOOLS.map(([k, namePt]) => ({ key: `proficiency:${k}`, namePt, kind: 'tool' as const })),
  ...LANGUAGES.map(([k, namePt]) => ({ key: `language:${k}`, namePt, kind: 'language' as const })),
].sort((a, b) => (a.kind === b.kind ? COLLATOR.compare(a.namePt, b.namePt) : a.kind === 'tool' ? -1 : 1));

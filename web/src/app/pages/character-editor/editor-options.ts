import type { CuttingWordsAskKey } from './character-editor.types';

/** The bard's "Perguntar" choices for Palavras de Interrupção, as the sheet says them. */
export const CUTTING_WORDS_OPTIONS: readonly {
  readonly value: CuttingWordsAskKey;
  readonly label: string;
}[] = [
  { value: 'all', label: 'Em todos os testes do inimigo' },
  { value: 'only-attacks', label: 'Só em ataques' },
  { value: 'never', label: 'Nunca' },
];

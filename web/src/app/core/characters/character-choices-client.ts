import { Injectable, inject } from '@angular/core';
import { clone } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';

import {
  type Character,
  CharacterService,
  type FullSheet,
  FullSheetSchema,
  type PreviewChoicesResponse,
} from '../../../gen/meurpg/characters/v1/characters_pb';
import { CONNECT_TRANSPORT } from '../connect/transport';

/** What the player answered on one open choice. */
export interface ChoiceAnswer {
  /** `Choice.key`. */
  readonly choiceKey: string;
  /** `ChoiceOption.key` of what was picked. */
  readonly optionKeys: readonly string[];
  /** The free texts, in order, for an option that takes them. */
  readonly texts: readonly string[];
}

/**
 * Thin wrapper around the calls that complete a sheet's open class and race choices (PM-05) on the generated
 * `CharacterService`. `providedIn: 'root'`, imported only by lazy code. The server decides every rule: which choices are
 * open, what an option gives and why one is blocked. Tests replace it with `{ provide: CharacterChoicesClient, useValue }`.
 */
@Injectable({ providedIn: 'root' })
export class CharacterChoicesClient {
  private readonly client = createClient(CharacterService, inject(CONNECT_TRANSPORT));

  async character(campaignId: string, characterId: string): Promise<Character> {
    const res = await this.client.getCharacter({ campaignId, characterId });
    return res.character!;
  }

  /**
   * `PreviewChoices` of a stored sheet with the picks so far laid over it: the server reads only the picks
   * (`feature_choice_keys`, `feature_choice_text`) of a sheet the caller may not edit, and the whole draft of one they may.
   */
  preview(
    campaignId: string,
    characterId: string,
    sheet: FullSheet,
    featureChoiceKeys: readonly string[],
    featureChoiceText: Readonly<Record<string, string>>,
  ): Promise<PreviewChoicesResponse> {
    const draft = clone(FullSheetSchema, sheet);
    draft.featureChoiceKeys = [...featureChoiceKeys];
    draft.featureChoiceText = { ...featureChoiceText };
    return this.client.previewChoices({
      campaignId,
      characterId,
      sheet: { content: { case: 'full', value: draft } },
    });
  }

  /** `CompleteCharacterChoices`: writes the picks of the open choices, and nothing else changes. */
  async complete(
    campaignId: string,
    characterId: string,
    expectedRevision: number,
    answers: readonly ChoiceAnswer[],
  ): Promise<Character> {
    const res = await this.client.completeCharacterChoices({
      campaignId,
      characterId,
      expectedRevision,
      picks: answers.map((a) => ({
        choiceKey: a.choiceKey,
        optionKeys: [...a.optionKeys],
        texts: [...a.texts],
      })),
    });
    return res.character!;
  }
}

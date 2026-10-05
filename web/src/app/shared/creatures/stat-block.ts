import { Component, computed, input } from '@angular/core';

import type { Creature, CreatureDamageModifier } from '../../../gen/meurpg/rules/v1/rules_pb';
import { signed } from '../../core/creatures/creature-format';
import { tight } from '../../core/format/text';
import { metersText } from '../../core/units';

/** The numbers of "PV": the creature's own (the owner's and the master's), or the book's. */
export interface StatBlockHp {
  readonly current: number;
  readonly max: number;
}

interface Tile {
  readonly label: string;
  readonly value: string;
  readonly of?: string;
}

/**
 * A creature's stat block as a sheet (E9-10, quadro 3), from `GetCreature`:
 * armor class and hit points, the speeds, the six abilities, the lines of
 * saves, skills, senses, languages and challenge, then the book's traits and
 * actions. The labels are ours, in Portuguese; the names and the text of the
 * traits and actions are the SRD's English, marked once ("Os textos abaixo são
 * do livro de regras (SRD 5.1), em inglês.") and carrying `lang="en"` for a
 * screen reader. The browser computes nothing: every number is what the
 * server sent.
 */
@Component({
  selector: 'app-stat-block',
  template: `
    @let c = creature();
    <section class="sb mr-panel" aria-label="Ficha da criatura">
      <div class="tiles">
        @for (t of tiles(); track t.label) {
          <div class="tile">
            <span class="tile__l">{{ t.label }}</span>
            <span class="tile__v">{{ t.value }}@if (t.of) {&nbsp;<small>{{ t.of }}</small>}</span>
          </div>
        }
      </div>

      <ul class="abilities" aria-label="Atributos">
        @for (a of c.abilities; track a.ability) {
          <li class="ability">
            <span class="ability__n">{{ a.namePt }}</span>
            <b class="ability__m">{{ signed(a.modifier) }}</b>
            <span class="ability__s"><span class="mr-visually-hidden">valor </span>{{ a.score }}</span>
          </li>
        }
      </ul>

      <dl class="lines">
        @for (l of lines(); track l.term) {
          <div class="line">
            <dt>{{ l.term }}</dt>
            <dd>
              @for (p of l.parts; track $index) {
                @if (p.en) {
                  <span lang="en">{{ p.text }}</span>
                } @else {
                  {{ p.text }}
                }
              }
            </dd>
          </div>
        }
      </dl>

      @if (groups().length > 0) {
        <p class="srd">Os textos abaixo são do livro de regras (SRD 5.1), em inglês.</p>
      }
      @for (g of groups(); track g.title) {
        <section class="group" [attr.aria-label]="g.title || 'Características'">
          @if (g.title) {
            <h2 class="group__t">{{ g.title }}</h2>
          }
          @for (e of g.entries; track $index) {
            <div class="entry" lang="en">
              <h3 class="entry__n">{{ e.name }}@if (e.usage) { <span class="entry__u">({{ e.usage }})</span> }</h3>
              <p class="entry__t">{{ e.text }}</p>
            </div>
          }
        </section>
      }
      <ng-content />
    </section>
  `,
  styleUrl: './stat-block.scss',
})
export class StatBlock {
  readonly creature = input.required<Creature>();
  /** The creature's own hit points; the book's (the average) when unset. */
  readonly hp = input<StatBlockHp | null>(null);

  protected readonly signed = signed;

  protected readonly tiles = computed<readonly Tile[]>(() => {
    const c = this.creature();
    const hp = this.hp();
    const out: Tile[] = [
      { label: 'CA', value: String(c.armorClass) },
      hp
        ? { label: 'PV', value: String(hp.current), of: `de ${hp.max}` }
        : { label: 'PV', value: String(c.hitPoints), of: c.hitPointsRoll ? `(${c.hitPointsRoll})` : '' },
    ];
    const speeds: [number, string][] = [
      [c.speedWalkFt, 'Deslocamento'],
      [c.speedFlyFt, 'Voo'],
      [c.speedSwimFt, 'Natação'],
      [c.speedClimbFt, 'Escalada'],
      [c.speedBurrowFt, 'Escavação'],
    ];
    const anySpeed = speeds.some(([ft]) => ft > 0);
    for (const [ft, label] of speeds) {
      if (ft > 0 || (!anySpeed && label === 'Deslocamento')) {
        out.push({ label, value: tight(metersText(ft)) });
      }
    }
    return out;
  });

  protected readonly lines = computed(() => {
    const c = this.creature();
    const out: { term: string; parts: { text: string; en?: boolean }[] }[] = [];
    const line = (term: string, text: string, en = false) => out.push({ term, parts: [{ text, en }] });
    if (c.savingThrows.length > 0) {
      line('Salvaguardas', c.savingThrows.map((b) => `${b.namePt} ${signed(b.bonus)}`).join(', '));
    }
    line('Perícias', c.skills.map((b) => `${b.namePt} ${signed(b.bonus)}`).join(', ') || '—');
    const modifiers: [string, readonly CreatureDamageModifier[]][] = [
      ['Vulnerável a', c.vulnerabilities],
      ['Resistente a', c.resistances],
      ['Imune a', c.immunities],
    ];
    for (const [term, list] of modifiers) {
      if (list.length > 0) {
        // The damage types are ours, in Portuguese; the book's note ("from nonmagical weapons") stays English.
        out.push({ term, parts: list.flatMap((m, i) => modifierParts(m, i > 0)) });
      }
    }
    if (c.conditionImmunities.length > 0) {
      line('Condições', `imune a ${c.conditionImmunities.map((k) => k.namePt).join(', ')}`);
    }
    const senses = c.senses.map((s) => tight(`${s.namePt} ${metersText(s.rangeFt)}`));
    line('Sentidos', [...senses, `Percepção passiva ${c.passivePerception}`].join(', '));
    line('Idiomas', c.languages || '—', c.languages !== '');
    line('Desafio', c.summary?.challengeRating ?? '0');
    return out;
  });

  protected readonly groups = computed(() => {
    const c = this.creature();
    const groups: { title: string; entries: readonly { name: string; text: string; usage: string }[] }[] = [];
    if (c.traits.length > 0) {
      groups.push({ title: 'Características', entries: c.traits });
    }
    if (c.actions.length > 0) {
      groups.push({ title: 'Ações', entries: c.actions });
    }
    if (c.reactions.length > 0) {
      groups.push({ title: 'Reações', entries: c.reactions });
    }
    if (c.legendaryActions.length > 0) {
      groups.push({ title: 'Ações lendárias', entries: c.legendaryActions });
    }
    return groups;
  });
}

/** "Fogo, Gelo" (ours) plus the book's note ("from nonmagical weapons", in English), after "; " when it is not the first. */
function modifierParts(m: CreatureDamageModifier, more: boolean): { text: string; en?: boolean }[] {
  const types = m.types.map((t) => t.namePt).join(', ');
  const out: { text: string; en?: boolean }[] = [];
  if (types) {
    out.push({ text: `${more ? '; ' : ''}${types}${m.note ? ' ' : ''}` });
  }
  if (m.note) {
    out.push({ text: `${types || !more ? '' : '; '}${m.note}`, en: true });
  }
  return out;
}

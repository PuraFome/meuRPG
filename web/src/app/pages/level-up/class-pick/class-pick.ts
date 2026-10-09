import {
  ChangeDetectionStrategy,
  Component,
  effect,
  ElementRef,
  computed,
  inject,
  input,
  output,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

import {
  LevelUpClassUnavailable,
  type LevelUpClassChoice,
  type LevelUpPrerequisite,
} from '../../../../gen/meurpg/characters/v1/characters_pb';
import { abilityLabel } from '../../../core/characters/character-labels';
import { ABILITY_FROM_GEN } from '../../../core/levelup/levelup-flow';

const COUNT = [
  'Nenhuma',
  'Uma',
  'Duas',
  'Três',
  'Quatro',
  'Cinco',
  'Seis',
  'Sete',
  'Oito',
  'Nove',
  'Dez',
  'Onze',
];

const LIST = new Intl.ListFormat('pt-BR', { type: 'conjunction' });
const OR_LIST = new Intl.ListFormat('pt-BR', { type: 'disjunction' });

function ability(p: LevelUpPrerequisite): string {
  const key = ABILITY_FROM_GEN[p.ability];
  return key ? abilityLabel(key) : '';
}

/** "Força 13", "Destreza 13 e Sabedoria 13", "Força 13 ou Destreza 13". */
export function requirementText(c: LevelUpClassChoice): string {
  const parts = c.prerequisites.map((p) => `${ability(p)} ${p.minimum}`);
  return c.prerequisiteAnyOf ? OR_LIST.format(parts) : LIST.format(parts);
}

/** What the character has in the abilities a class asks for: "Força 16", "Força 12, Destreza 12". */
function haveText(c: LevelUpClassChoice, only?: (p: LevelUpPrerequisite) => boolean): string {
  return c.prerequisites
    .filter((p) => !only || only(p))
    .map((p) => `${ability(p)} ${p.have}`)
    .join(', ');
}

/**
 * "Subir em qual classe?": the first step of the guided level-up, always (MR-040, SRD 5.1
 * "Multiclassing"). One radio card per class the character has, each with "nível 5 → 6", and "Uma classe nova",
 * which opens the other classes as cards with what each asks for ("Exige Inteligência 13. Você tem
 * Inteligência 13."). A class the character does not qualify for stays in the list, dashed, with the reason
 * in words and `aria-disabled` (it keeps the focus, so a screen reader reads why). Tapping another class after
 * choices were made does not switch at once: the question "Trocar de classe?" opens in place, with the safe
 * button, "Continuar com ...", first to get the focus. The page reads the options and runs the switch.
 */
@Component({
  selector: 'app-class-pick',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatButtonModule, MatIconModule],
  templateUrl: './class-pick.html',
  styleUrl: './class-pick.scss',
})
export class ClassPick {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** Every way the level can go, as the server lists them: the classes the character has, then the new ones. */
  readonly choices = input.required<readonly LevelUpClassChoice[]>();
  /** The class the level goes to now (the options below are its). */
  readonly selected = input.required<string>();
  /** "Uma classe nova" is open: the cards of the new classes show. */
  readonly newMode = input(false);
  /** The character's total level after the level-up. */
  readonly totalToLevel = input.required<number>();
  /** The options of the other class are being read: the cards wait. */
  readonly busy = input(false);
  /** The class the person tapped while choices were made: the question is open for it. */
  readonly asking = input<string | null>(null);
  /** The words a screen reader hears when the class changes. */
  readonly status = input('');

  constructor() {
    // The checked radios follow what the page says, whenever it changes (an answer that comes late included).
    effect(() => {
      this.selected();
      this.newMode();
      queueMicrotask(() => this.syncRadios());
    });
  }

  readonly pick = output<string>();
  readonly pickNew = output<void>();
  readonly confirmSwitch = output<void>();
  readonly keep = output<void>();

  protected readonly have = computed(() => this.choices().filter((c) => !c.isNew));
  protected readonly fresh = computed(() => this.choices().filter((c) => c.isNew));
  protected readonly open = computed(() => this.fresh().filter((c) => c.available));
  protected readonly asked = computed(() =>
    this.choices().find((c) => c.classKey === this.asking()),
  );
  protected readonly current = computed(() =>
    this.choices().find((c) => c.classKey === this.selected()),
  );
  /** The classes the character has whose own prerequisite it does not meet: they close every new class. */
  protected readonly unmetCurrent = computed(() => this.have().filter((c) => !c.prerequisiteMet));
  protected readonly atCap = computed(
    () =>
      this.fresh().length > 0 &&
      this.fresh().every((c) => c.unavailable === LevelUpClassUnavailable.MAX_LEVEL),
  );
  /** "Uma classe nova" can be chosen: at least one new class is open. */
  protected readonly newOpen = computed(() => this.open().length > 0);
  protected readonly newCount = computed(
    () => COUNT[this.open().length] ?? String(this.open().length),
  );

  /** The sentence under "Uma classe nova": the prerequisite of the classes the character has, in words. */
  protected readonly lead = computed(() => {
    const classes = this.have();
    if (this.atCap()) {
      return 'O personagem já está no nível 20: não há classe nova.';
    }
    const unmet = this.unmetCurrent();
    const others = LIST.format(classes.slice(1).map((c) => c.namePt));
    if (unmet.length > 0) {
      const c = unmet[0];
      return `Você não cumpre o pré-requisito d${article(c.namePt)} ${c.namePt}: ${requirementText(c)} (${haveText(c)}). Uma classe nova só entra com o pré-requisito ${classes.length > 1 ? 'de todas as classes' : 'das duas'}.`;
    }
    const first = classes[0];
    const rest = classes.length > 1 ? ` e o das outras classes (${others})` : '';
    return `Você cumpre o pré-requisito d${article(first.namePt)} ${first.namePt}: ${requirementText(first)} (${haveText(first)})${rest}. Para uma classe nova, é preciso cumprir o ${classes.length > 1 ? 'de todas' : 'das duas'}.`;
  });

  protected readonly newIntro = computed(() => {
    const n = this.open().length;
    return n === 0
      ? 'Nenhuma está ao seu alcance: todas exigem uma habilidade que você ainda não tem.'
      : `${this.newCount()} ${n === 1 ? 'está' : 'estão'} ao seu alcance. As outras exigem uma habilidade que você ainda não tem.`;
  });

  protected next(c: LevelUpClassChoice): string {
    const parts = [`nível ${c.fromLevel} → ${c.toLevel}`];
    if (c.subclassNamePt) parts.push(c.subclassNamePt);
    if (c.subclassDue) parts.push('escolhe a subclasse');
    return parts.join(' · ');
  }

  protected exige(c: LevelUpClassChoice): string {
    return `Exige ${requirementText(c)}.`;
  }

  /** "Você tem Inteligência 13." for a class that qualifies; "Falta: Carisma 13 (você tem 9)." for one that does not. */
  protected reason(c: LevelUpClassChoice): string {
    if (c.prerequisiteMet) {
      return `Você tem ${haveText(c, c.prerequisiteAnyOf ? (p) => p.met : undefined)}.`;
    }
    const missing = c.prerequisites.filter((p) => !p.met);
    return `Falta: ${missing.map((p) => `${ability(p)} ${p.minimum} (você tem ${p.have})`).join(', ')}.`;
  }

  /** The new class cards that cannot be chosen: dashed, with the reason. */
  protected closed(c: LevelUpClassChoice): boolean {
    return !c.available;
  }

  protected chooseNew(c: LevelUpClassChoice): void {
    if (c.available) {
      this.pick.emit(c.classKey);
    }
    // A closed card (aria-disabled) is focusable, and the browser still checks it on Space: it goes back at once.
    queueMicrotask(() => this.syncRadios());
    setTimeout(() => this.syncRadios());
  }

  protected choose(key: string): void {
    this.pick.emit(key);
    // A card the page did not take (the question opened instead) goes back to the class that is still in force.
    queueMicrotask(() => this.syncRadios());
    setTimeout(() => this.syncRadios());
  }

  protected chooseNewMode(): void {
    if (this.newOpen()) {
      this.pickNew.emit();
    }
    queueMicrotask(() => this.syncRadios());
    setTimeout(() => this.syncRadios());
  }

  /** The checked radios always follow the page's state, whatever the person tapped. */
  private syncRadios(): void {
    const root = this.host.nativeElement;
    root.querySelectorAll<HTMLInputElement>('input[name=level-up-class]').forEach((i) => {
      i.checked = this.newMode() ? i.value === '' : i.value === this.selected();
    });
    root.querySelectorAll<HTMLInputElement>('input[name=level-up-new-class]').forEach((i) => {
      i.checked = i.value === this.selected();
    });
  }

  /** The focus goes to the checked card: where the page opens, and where it returns after a switch. */
  focusSelected(): void {
    const root = this.host.nativeElement;
    (
      root.querySelector<HTMLInputElement>('input[name=level-up-new-class]:checked') ??
      root.querySelector<HTMLInputElement>('input[name=level-up-class]:checked')
    )?.focus();
  }
}

/** "o" or "a": "do Mago", "da Bruxa"... */
function article(name: string): string {
  return /^(Bruxa|Druida)$/.test(name) ? 'a' : 'o';
}

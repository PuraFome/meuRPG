import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { RouterLink } from '@angular/router';

import type { CreatureSummary, Creature } from '../../../../gen/meurpg/rules/v1/rules_pb';
import { type TableEntry } from '../../../../gen/meurpg/rules/v1/table_content_pb';
import { type CatalogVm } from '../../../core/content/catalog';
import { type EntryBody, TableContentClient } from '../../../core/content/content-client';
import {
  type CreatureAttackDraft,
  type CreatureDraft,
  type CreatureTraitDraft,
  DICE_SIDES,
  MAX_ATTACKS,
  MAX_TRAITS,
  UNREADABLE_RANGE,
  creaturePaths,
  creatureToDraft,
  draftToCreature,
  emptyAttack,
  emptyCreature,
  emptyTrait,
  unreadableCreaturePaths,
} from '../../../core/content/creature-draft';
import { carriersOf } from '../../../core/content/creature-carriers';
import { creaturePreview } from '../../../core/content/creature-preview';
import { RANGES, attackSummary, formulaEcho, traitLine } from '../../../core/content/creature-text';
import { EntrySaver, focusAfterRender } from '../../../core/content/entry-saver';
import { UNREADABLE_BONUS, UNREADABLE_COUNT } from '../../../core/content/whole-number';
import { creatureSlug, typeAndSize } from '../../../core/creatures/bestiary-format';
import { CreaturesClient } from '../../../core/creatures/creatures-client';
import { CreaturePick } from '../../../shared/creature-pick/creature-pick';
import { SelectField, type SelectOption } from '../../../shared/form-fields/select-field';
import { TextField } from '../../../shared/form-fields/text-field';
import { TableMark } from '../../../shared/table-mark/table-mark';
import { EditorAlerts, EditorBar } from '../editor-bar/editor-bar';
import { PlayersSwitch } from '../players-switch/players-switch';
import type { EditorSaved } from '../spell-editor/spell-editor';

const P = 'table_creature';

/** One of the four numbers of "Números": a fixed value and a formula of the owner's sheet (the attack bonus has no fixed one). */
interface NumberRow {
  readonly label: string;
  readonly path: string;
  readonly fixed: 'hitPoints' | 'armorClass' | 'damageBonus' | null;
  readonly formula:
    'hitPointsFormula' | 'armorClassFormula' | 'damageBonusFormula' | 'attackBonusFormula';
  readonly range: string;
}

const NUMBER_ROWS: readonly NumberRow[] = [
  {
    label: 'Pontos de vida',
    path: 'hit_points',
    fixed: 'hitPoints',
    formula: 'hitPointsFormula',
    range: RANGES.hitPoints,
  },
  {
    label: 'Classe de armadura',
    path: 'armor_class',
    fixed: 'armorClass',
    formula: 'armorClassFormula',
    range: RANGES.armorClass,
  },
  {
    label: 'Bônus no dano',
    path: 'damage_bonus',
    fixed: 'damageBonus',
    formula: 'damageBonusFormula',
    range: RANGES.damageBonus,
  },
  {
    label: 'Bônus de ataque',
    path: 'attack_bonus',
    fixed: null,
    formula: 'attackBonusFormula',
    range: RANGES.attackBonus,
  },
];

/**
 * The table creature editor (#578): the master makes a creature of the table from an SRD creature (the base, chosen once and
 * read-only after the first save), with its own name, four numbers (a fixed value and a formula of the owner's sheet), traits (text
 * only) and attacks. The side panel writes "Como fica na mesa" from the draft and the base's stat block, and lists what brings the
 * creature to a character. The resistances, immunities and vulnerabilities have no rows yet: the draft keeps them as the entry has
 * them (`kept`). "Salvar criatura" sends the whole entry; the server says what is wrong, field by field.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-creature-editor',
  imports: [
    CreaturePick,
    EditorAlerts,
    EditorBar,
    MatButtonModule,
    MatIconModule,
    MatMenuModule,
    PlayersSwitch,
    RouterLink,
    SelectField,
    TableMark,
    TextField,
  ],
  templateUrl: './creature-editor.html',
  styleUrls: ['../editor.scss', './creature-editor.scss'],
})
export class CreatureEditor {
  private readonly client = inject(TableContentClient);
  private readonly creatures = inject(CreaturesClient);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly campaignId = input.required<string>();
  readonly entry = input<TableEntry | null>(null);
  readonly catalog = input.required<CatalogVm>();
  readonly entries = input<readonly TableEntry[]>([]);
  readonly saveBlocked = input('');

  readonly saved = output<EditorSaved>();
  /** The entry's switch "Para os jogadores" was turned (it saves at once, apart from the form). */
  readonly switched = output<TableEntry>();
  readonly reload = output<void>();
  readonly cancelled = output<void>();

  protected readonly draft = signal<CreatureDraft>(emptyCreature());
  protected readonly saver = new EntrySaver(
    {
      aOne: 'uma criatura',
      nameOf: (key) => this.entries().find((e) => e.key === key)?.namePt ?? '',
    },
    'a criatura',
  );
  protected readonly isNew = computed(() => this.entry() === null);
  /** The base's stat block, once read; `null` before, and when it could not be read. */
  protected readonly base = signal<Creature | null>(null);
  /** The name of the base the master just picked, until the stat block says it. */
  private readonly pickedName = signal('');
  /** The rows (by id) the master opened; a row with a refusal is open as well. */
  protected readonly open = signal<ReadonlySet<string>>(new Set());
  /** What the last add or remove did, for a screen reader. */
  protected readonly status = signal('');

  protected readonly numberRows = NUMBER_ROWS;
  protected readonly diceSides: readonly SelectOption[] = DICE_SIDES.map((n) => ({
    value: n,
    label: `d${n}`,
  }));
  protected readonly shapes: readonly SelectOption[] = [
    { value: 'melee', label: 'Corpo a corpo' },
    { value: 'ranged', label: 'À distância' },
  ];
  protected readonly maxAttacks = MAX_ATTACKS;
  protected readonly maxTraits = MAX_TRAITS;
  protected readonly damageTypes = computed(() => this.catalog().damageTypes);

  private readonly baseKey = computed(() => this.draft().baseKey);
  protected readonly baseName = computed(
    () => this.base()?.summary?.namePt || this.pickedName() || creatureSlug(this.baseKey()),
  );
  /** "Lobo, do SRD (Fera · Médio · ND 1/4)". */
  protected readonly baseLine = computed(() => {
    const s = this.base()?.summary;
    const facts = s ? [typeAndSize(s), `ND ${s.challengeRating}`].filter((x) => x).join(' · ') : '';
    return `${this.baseName()}, do SRD${facts ? ` (${facts})` : ''}`;
  });
  protected readonly baseLink = computed(() => [
    '/campaigns',
    this.campaignId(),
    'bestiary',
    creatureSlug(this.baseKey()),
  ]);
  protected readonly preview = computed(() =>
    creaturePreview(this.draft(), this.base(), this.baseName(), (k) => this.catalog().nameOf(k)),
  );
  protected readonly carriers = computed(() => carriersOf(this.entries(), this.entry()?.key ?? ''));
  protected readonly attacksFull = computed(() => this.draft().attacks.length >= MAX_ATTACKS);
  protected readonly traitsFull = computed(() => this.draft().traits.length >= MAX_TRAITS);

  constructor() {
    // Keyed on the entry's key and revision, never on the object: archiving hands the page a new object with the same revision,
    // and the master's unsaved draft must stay.
    const source = computed(() => {
      const e = this.entry();
      return e ? `${e.key}@${e.revision}` : 'new';
    });
    effect(() => {
      source();
      untracked(() => {
        const e = this.entry();
        this.draft.set(
          e?.body.case === 'tableCreature' ? creatureToDraft(e.body.value) : emptyCreature(),
        );
        this.open.set(new Set());
        this.saver.clear();
      });
    });
    // The base's stat block, for the preview and the placeholders: read again only when the base changes.
    effect(() => {
      const key = this.baseKey();
      untracked(() => void this.readBase(key));
    });
  }

  private seq = 0;

  private async readBase(key: string): Promise<void> {
    const mine = ++this.seq;
    if (key === '') {
      this.base.set(null);
      return;
    }
    try {
      const block = await this.creatures.statBlock(this.campaignId(), key);
      if (mine === this.seq) {
        this.base.set(block);
      }
    } catch {
      if (mine === this.seq) {
        this.base.set(null);
      }
    }
  }

  protected patch(p: Partial<CreatureDraft>): void {
    if (this.unreadable().size > 0) {
      this.unreadable.set(new Map());
    }
    this.draft.update((d) => ({ ...d, ...p }));
  }

  protected setNumber(key: NumberRow['fixed'] | NumberRow['formula'], value: string): void {
    if (key) {
      this.patch({ [key]: value });
    }
  }

  protected numberText(key: NumberRow['fixed'] | NumberRow['formula']): string {
    return key ? (this.draft()[key] as string) : '';
  }

  protected fixedPlaceholder(r: NumberRow): string {
    const b = this.base();
    switch (r.fixed) {
      case 'hitPoints':
        return b ? `${b.hitPoints}, da base` : 'da base';
      case 'armorClass':
        return b ? `${b.armorClass}, da base` : 'da base';
      default:
        return '0';
    }
  }

  protected echo(r: NumberRow): string {
    return formulaEcho(this.draft()[r.formula], r.range);
  }

  protected pickBase(c: CreatureSummary): void {
    this.pickedName.set(c.namePt);
    this.patch({ baseKey: c.key });
    focusAfterRender(this.host.nativeElement, this.injector, `${P}.name_pt`);
  }

  /** A new creature may still change its base; a saved one never does (the server refuses it). */
  protected changeBase(): void {
    this.pickedName.set('');
    this.patch({ baseKey: '' });
  }

  private hasIssue(prefix: string): boolean {
    return this.saver.placement().fields.some((f) => f === prefix || f.startsWith(`${prefix}.`));
  }

  protected isOpen(id: string, path: string): boolean {
    return this.open().has(id) || this.hasIssue(path);
  }

  protected toggle(id: string): void {
    this.open.update((s) => {
      const next = new Set(s);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
  }

  private show(id: string): void {
    this.open.update((s) => new Set(s).add(id));
  }

  protected traitLine = traitLine;
  protected attackLine(a: CreatureAttackDraft): string {
    return attackSummary(a, (k) => this.catalog().nameOf(k));
  }

  protected addTrait(): void {
    if (this.traitsFull()) {
      return;
    }
    const t = emptyTrait();
    const at = this.draft().traits.length;
    this.patch({ traits: [...this.draft().traits, t] });
    this.show(t.id);
    this.status.set('Traço adicionado.');
    focusAfterRender(this.host.nativeElement, this.injector, `${P}.traits[${at}].name_pt`);
  }

  protected patchTrait(id: string, p: Partial<CreatureTraitDraft>): void {
    this.patch({ traits: this.draft().traits.map((t) => (t.id === id ? { ...t, ...p } : t)) });
  }

  protected removeTrait(id: string): void {
    this.patch({ traits: this.draft().traits.filter((t) => t.id !== id) });
    // The refusals name rows by their place: after a row goes, they would sit on the wrong one.
    this.saver.clear();
    this.status.set('Traço removido.');
  }

  protected addAttack(): void {
    if (this.attacksFull()) {
      return;
    }
    const a = emptyAttack();
    const at = this.draft().attacks.length;
    this.patch({ attacks: [...this.draft().attacks, a] });
    this.show(a.id);
    this.status.set('Ataque adicionado.');
    focusAfterRender(this.host.nativeElement, this.injector, `${P}.attacks[${at}].name_pt`);
  }

  protected patchAttack(id: string, p: Partial<CreatureAttackDraft>): void {
    this.patch({ attacks: this.draft().attacks.map((a) => (a.id === id ? { ...a, ...p } : a)) });
  }

  protected removeAttack(id: string): void {
    this.patch({ attacks: this.draft().attacks.filter((a) => a.id !== id) });
    this.saver.clear();
    this.status.set('Ataque removido.');
  }

  protected shapeOf(a: CreatureAttackDraft): string {
    return a.melee ? 'melee' : 'ranged';
  }

  private readonly known = (path: string): boolean => creaturePaths(this.draft()).includes(path);

  /** What the client says about a number it cannot read, by path: the server's refusals are for what it could read. */
  protected readonly unreadable = signal<ReadonlyMap<string, string>>(new Map());

  /** The messages of one field: the client's own first, then the server's. */
  protected messages(path: string): readonly string[] {
    const own = this.unreadable().get(path);
    return own ? [own] : this.saver.issues(path);
  }

  private readonly shapeOfPath = (path: string): string => {
    if (/\.(reach_ft|range_ft|long_range_ft)$/.test(path)) {
      return UNREADABLE_RANGE;
    }
    return path === `${P}.damage_bonus` ? UNREADABLE_BONUS : UNREADABLE_COUNT;
  };

  /** Opens the attack row that holds a path, so its field can be focused. */
  private openRowOf(path: string): void {
    const m = /^table_creature\.attacks\[(\d+)\]/.exec(path);
    const row = m ? this.draft().attacks[Number(m[1])] : undefined;
    if (row) {
      this.show(row.id);
    }
  }

  protected async save(): Promise<void> {
    if (this.saver.saving() || this.saveBlocked()) {
      return;
    }
    const bad = unreadableCreaturePaths(this.draft());
    this.unreadable.set(new Map(bad.map((p) => [p, this.shapeOfPath(p)])));
    if (bad.length > 0) {
      this.saver.clear();
      bad.forEach((p) => this.openRowOf(p));
      focusAfterRender(this.host.nativeElement, this.injector, bad[0]);
      return;
    }
    const body: EntryBody = { case: 'tableCreature', value: draftToCreature(this.draft()) };
    const res = await this.saver.run(
      () => this.client.save(this.campaignId(), this.entry(), body, this.saver.keyFor(body)),
      this.known,
    );
    if (res) {
      this.saved.emit(res);
      return;
    }
    this.saver.focusRefusal(this.host.nativeElement, this.injector);
  }
}

import {
  Component,
  ElementRef,
  Injector,
  OnInit,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';

import type { SceneAction } from '../../../../gen/meurpg/maps/v1/maps_pb';
import {
  CHECK_KINDS,
  type CheckKind,
  type CheckOption,
  SCENE_ACTION_NAME_MAX,
  SCENE_DC_MAX,
  SCENE_DC_MIN,
  SceneChecks,
  checkOptions,
  parseDc,
} from '../../../core/maps/scene-actions';
import { tight } from '../../../core/format/text';
import { FictionNotice } from '../../../shared/fiction-notice/fiction-notice';

let nextId = 0;

/** What the form hands the panel when it is valid. */
export interface NewSceneAction {
  readonly key: string;
  readonly name: string;
  readonly dc: number;
}

/** The fields of an edited action that changed: what `UpdateSceneAction` is sent (`name` empty takes it off, `dc` 0 too). */
export interface SceneActionChanges {
  readonly key?: string;
  readonly name?: string;
  readonly dc?: number;
}

/** The kind of a check by its key: "ability:str" and "save:wis" say it, every skill key (the rules' or the table's) is a skill. */
function kindOf(key: string): CheckKind {
  return key.startsWith('ability:') ? 'ability' : key.startsWith('save:') ? 'save' : 'skill';
}

/**
 * "Nova ação" (E7-01): the form that opens in place inside the point panel.
 * "O que rolar" (skill, ability check or saving throw) picks the list of the
 * next field; the name (up to 60 characters) and the DC (1 to 30) are
 * optional. The buttons are under the fields. A DC out of range is said under
 * its field, with icon and words, and focus goes to it. The panel runs the
 * call and shows what the server answered in `error`.
 *
 * With `initial` it is "Editar ação": the same card in the place of the action's row, filled with its
 * check, name and DC; "Salvar ação" stays dashed, with its reason, until a field differs from what the
 * action has, and then hands the panel only the fields that changed.
 */
@Component({
  selector: 'app-scene-action-form',
  imports: [
    FictionNotice,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    ReactiveFormsModule,
  ],
  templateUrl: './scene-action-form.html',
  styleUrl: './scene-action-form.scss',
})
export class SceneActionForm implements OnInit {
  private readonly checks = inject(SceneChecks);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  readonly campaignId = input.required<string>();
  /** An answer the server refused (the 20-action limit, a lost point). */
  readonly error = input('');
  readonly busy = input(false);
  /** The action being edited; none means a new one. */
  readonly initial = input<SceneAction | null>(null);

  readonly submitted = output<NewSceneAction>();
  /** "Salvar ação": the fields that changed. */
  readonly saved = output<SceneActionChanges>();
  readonly cancelled = output<void>();

  protected readonly id = `sa-form-${nextId++}`;
  protected readonly kinds = CHECK_KINDS;
  protected readonly nameMax = SCENE_ACTION_NAME_MAX;
  protected readonly kind = signal<CheckKind>('skill');
  protected readonly checkKey = signal('');
  protected readonly skills = signal<readonly CheckOption[] | null>(null);
  protected readonly skillsFailed = signal(false);
  protected readonly dcError = signal('');
  /** "Escolha ...": a new action submitted with no check picked (nothing is preselected). */
  protected readonly checkError = signal('');
  protected readonly nameControl = new FormControl('', { nonNullable: true });
  protected readonly dcControl = new FormControl('', { nonNullable: true });
  protected readonly nameLength = signal(0);
  private readonly nameText = signal('');
  private readonly dcText = signal('');

  /** An edit with no field changed: nothing to save (the server would refuse it as having nothing to change). */
  protected readonly unchanged = computed(() => {
    const a = this.initial();
    return a !== null && this.changes() === null;
  });
  private readonly changes = computed<SceneActionChanges | null>(() => {
    const a = this.initial();
    if (!a) {
      return null;
    }
    const dc = parseDc(this.dcText());
    const out: { key?: string; name?: string; dc?: number } = {};
    if (this.checkKey() !== a.key) {
      out.key = this.checkKey();
    }
    if (this.nameText().trim() !== a.name) {
      out.name = this.nameText().trim();
    }
    // A DC out of range counts as a change: saving says why it is not accepted.
    if (dc === null || dc !== a.dc) {
      out.dc = dc ?? 0;
    }
    return Object.keys(out).length > 0 ? out : null;
  });

  protected readonly options = computed(() => {
    const list = checkOptions(this.kind(), this.skills() ?? []);
    const a = this.initial();
    // The check the action has stays on the list even if the table retired it.
    return a && this.kind() === kindOf(a.key) && !list.some((o) => o.key === a.key)
      ? [{ key: a.key, label: a.checkName }, ...list]
      : list;
  });
  protected readonly kindLabel = computed(
    () => this.kinds.find((k) => k.kind === this.kind())?.label ?? '',
  );
  protected readonly dcMessage = tight(
    `A CD vai de ${SCENE_DC_MIN} a ${SCENE_DC_MAX}. Digite outro número ou deixe em branco.`,
  );

  private readonly firstRadio = viewChild('firstRadio', { read: ElementRef<HTMLInputElement> });
  private readonly checkField = viewChild('checkField', { read: ElementRef<HTMLSelectElement> });
  private readonly dcField = viewChild('dcField', { read: ElementRef<HTMLInputElement> });

  constructor() {
    this.nameControl.valueChanges.pipe(takeUntilDestroyed()).subscribe((v) => {
      this.nameLength.set(v.length);
      this.nameText.set(v);
    });
    this.dcControl.valueChanges.pipe(takeUntilDestroyed()).subscribe((v) => {
      this.dcText.set(v);
      if (this.dcError()) {
        this.dcError.set('');
        this.dcControl.setErrors(null);
      }
    });
    afterNextRender(
      () => {
        // The whole form comes into view (the panel scrolls beside a sticky map), then
        // the focus goes to the first choice without scrolling again.
        this.host.nativeElement.scrollIntoView({ block: 'nearest' });
        this.firstRadio()?.nativeElement.focus({ preventScroll: true });
      },
      { injector: this.injector },
    );
  }

  /** The skills load once the inputs are set (the campaign is one of them). */
  ngOnInit(): void {
    const a = this.initial();
    if (a) {
      this.kind.set(kindOf(a.key));
      this.checkKey.set(a.key);
      this.nameControl.setValue(a.name);
      this.dcControl.setValue(a.dc > 0 ? String(a.dc) : '');
    }
    this.loadSkills();
  }

  protected loadSkills(): void {
    this.skillsFailed.set(false);
    this.checks.skills(this.campaignId()).then(
      (skills) => {
        this.skills.set(skills);
        // Editing: the list loads under the check the action already has. A new
        // action picks nothing: the master chooses.
      },
      () => this.skillsFailed.set(true),
    );
  }

  protected pickKind(kind: CheckKind): void {
    this.kind.set(kind);
    this.checkError.set('');
    if (this.initial()) {
      this.pickFirst();
    } else {
      this.checkKey.set('');
    }
  }

  protected pickCheck(key: string): void {
    this.checkKey.set(key);
    this.checkError.set('');
  }

  private pickFirst(): void {
    this.checkKey.set(this.options()[0]?.key ?? '');
  }

  protected submit(): void {
    if (this.busy()) {
      return;
    }
    const dc = parseDc(this.dcControl.value);
    if (dc === null) {
      this.dcError.set(this.dcMessage);
      this.dcControl.setErrors({ dc: true });
      this.dcControl.markAsTouched();
      this.dcField()?.nativeElement.focus();
      return;
    }
    if (!this.checkKey()) {
      this.checkError.set(
        `Escolha ${this.kind() === 'skill' ? 'a perícia' : this.kind() === 'ability' ? 'a habilidade' : 'o teste de resistência'} a rolar.`,
      );
      this.checkField()?.nativeElement.focus();
      return;
    }
    if (this.initial()) {
      const changes = this.changes();
      if (changes) {
        this.saved.emit(changes);
      }
      return;
    }
    this.submitted.emit({ key: this.checkKey(), name: this.nameControl.value.trim(), dc });
  }
}

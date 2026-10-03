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

/**
 * "Nova ação" (E7-01): the form that opens in place inside the point panel.
 * "O que rolar" (skill, ability check or saving throw) picks the list of the
 * next field; the name (up to 60 characters) and the DC (1 to 30) are
 * optional. The buttons are under the fields. A DC out of range is said under
 * its field, with icon and words, and focus goes to it. The panel runs the
 * call and shows what the server answered in `error`.
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

  readonly submitted = output<NewSceneAction>();
  readonly cancelled = output<void>();

  protected readonly id = `sa-form-${nextId++}`;
  protected readonly kinds = CHECK_KINDS;
  protected readonly nameMax = SCENE_ACTION_NAME_MAX;
  protected readonly kind = signal<CheckKind>('skill');
  protected readonly checkKey = signal('');
  protected readonly skills = signal<readonly CheckOption[] | null>(null);
  protected readonly skillsFailed = signal(false);
  protected readonly dcError = signal('');
  protected readonly nameControl = new FormControl('', { nonNullable: true });
  protected readonly dcControl = new FormControl('', { nonNullable: true });
  protected readonly nameLength = signal(0);

  protected readonly options = computed(() => checkOptions(this.kind(), this.skills() ?? []));
  protected readonly kindLabel = computed(
    () => this.kinds.find((k) => k.kind === this.kind())?.label ?? '',
  );
  protected readonly dcMessage = tight(
    `A CD vai de ${SCENE_DC_MIN} a ${SCENE_DC_MAX}. Digite outro número ou deixe em branco.`,
  );

  private readonly firstRadio = viewChild('firstRadio', { read: ElementRef<HTMLInputElement> });
  private readonly dcField = viewChild('dcField', { read: ElementRef<HTMLInputElement> });

  constructor() {
    this.nameControl.valueChanges
      .pipe(takeUntilDestroyed())
      .subscribe((v) => this.nameLength.set(v.length));
    this.dcControl.valueChanges.pipe(takeUntilDestroyed()).subscribe(() => {
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
    this.loadSkills();
  }

  protected loadSkills(): void {
    this.skillsFailed.set(false);
    this.checks.skills(this.campaignId()).then(
      (skills) => {
        this.skills.set(skills);
        this.pickFirst();
      },
      () => this.skillsFailed.set(true),
    );
  }

  protected pickKind(kind: CheckKind): void {
    this.kind.set(kind);
    this.pickFirst();
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
      return;
    }
    this.submitted.emit({ key: this.checkKey(), name: this.nameControl.value.trim(), dc });
  }
}

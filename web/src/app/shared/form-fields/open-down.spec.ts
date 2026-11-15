import { ChangeDetectionStrategy, Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { MatSelect, MatSelectModule } from '@angular/material/select';
import { By } from '@angular/platform-browser';
import { OpenDown } from './open-down';
import { MatFormFieldModule } from '@angular/material/form-field';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [MatFormFieldModule, MatSelectModule, OpenDown],
  template: `<mat-form-field><mat-label>Armas</mat-label><mat-select appOpenDown multiple><mat-option value="a">A</mat-option></mat-select></mat-form-field>`,
})
class Host {}

describe('OpenDown', () => {
  function setup(selectBottom: number) {
    const fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    const select = fixture.debugElement.query(By.directive(MatSelect));
    const field = fixture.nativeElement.querySelector('mat-form-field') as HTMLElement;
    vi.spyOn(select.nativeElement, 'getBoundingClientRect').mockReturnValue({
      bottom: selectBottom,
    } as DOMRect);
    return { select: select.componentInstance as MatSelect, field };
  }

  it('keeps only the positions under the field', () => {
    const { select } = setup(100);
    expect(select._positions.length).toBeGreaterThan(0);
    expect(select._positions.every((p) => p.originY === 'bottom' && p.overlayY === 'top')).toBe(
      true,
    );
  });

  it('brings the field to the middle of the screen when the room below is short, and leaves it when there is room', () => {
    vi.stubGlobal('innerHeight', 900);
    const near = setup(800);
    near.select.openedChange.emit(true);
    expect(vi.mocked(Element.prototype.scrollIntoView)).toHaveBeenCalledWith({ block: 'center' });

    vi.mocked(Element.prototype.scrollIntoView).mockClear();
    const far = setup(200);
    far.select.openedChange.emit(true);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });
});

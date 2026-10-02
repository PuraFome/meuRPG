import { Component, ElementRef, output, signal, viewChildren } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';

export type ToolbarAction = 'heading' | 'bold' | 'italic' | 'list' | 'image' | 'map' | 'sheet';

interface Tool {
  readonly action: ToolbarAction;
  readonly label: string;
  readonly icon: string;
  /** A rule before the tool (the group of "insert from the campaign"). */
  readonly divider?: boolean;
  /** Opens a dialog. */
  readonly dialog?: boolean;
}

const TOOLS: readonly Tool[] = [
  { action: 'heading', label: 'Título', icon: 'title' },
  { action: 'bold', label: 'Negrito', icon: 'format_bold' },
  { action: 'italic', label: 'Itálico', icon: 'format_italic' },
  { action: 'list', label: 'Lista', icon: 'format_list_bulleted' },
  { action: 'image', label: 'Imagem da galeria', icon: 'image', divider: true, dialog: true },
  { action: 'map', label: 'Link para mapa', icon: 'map', dialog: true },
  { action: 'sheet', label: 'Link para ficha', icon: 'badge', dialog: true },
];

/**
 * The editor's toolbar (E5-28): a real `role="toolbar"`. Tab enters on one
 * button (the last one used) and leaves again; ← → (and Home/End) move
 * between the buttons, as the WAI-ARIA toolbar pattern says. Each button
 * says what it does; the icons are decoration.
 *
 * Mousedown on a button does not take focus from the text box, so the
 * selection stays where it was; the editor reads it when `act` fires.
 */
@Component({
  selector: 'app-document-toolbar',
  imports: [MatIconModule],
  templateUrl: './document-toolbar.html',
  styleUrl: './document-toolbar.scss',
})
export class DocumentToolbar {
  readonly act = output<{ action: ToolbarAction; trigger: HTMLElement }>();

  protected readonly tools = TOOLS;
  protected readonly active = signal(0);
  private readonly buttons = viewChildren<ElementRef<HTMLButtonElement>>('btn');

  protected onKeydown(event: KeyboardEvent, index: number): void {
    const last = this.tools.length - 1;
    let target: number;
    switch (event.key) {
      case 'ArrowRight':
        target = index === last ? 0 : index + 1;
        break;
      case 'ArrowLeft':
        target = index === 0 ? last : index - 1;
        break;
      case 'Home':
        target = 0;
        break;
      case 'End':
        target = last;
        break;
      default:
        return;
    }
    event.preventDefault();
    this.active.set(target);
    this.buttons()[target]?.nativeElement.focus();
  }

  protected press(tool: Tool, index: number, button: HTMLButtonElement): void {
    this.active.set(index);
    this.act.emit({ action: tool.action, trigger: button });
  }
}

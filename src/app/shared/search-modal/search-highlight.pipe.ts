import { Pipe, PipeTransform, inject } from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

@Pipe({
  name: 'searchHighlight',
  standalone: true,
})
export class SearchHighlightPipe implements PipeTransform {
  private readonly sanitizer = inject(DomSanitizer);

  transform(text: string, query: string): SafeHtml {
    if (!text) {
      return this.sanitizer.bypassSecurityTrustHtml('');
    }
    if (!query) {
      return this.sanitizer.bypassSecurityTrustHtml(text);
    }
    // Escape regex special characters to prevent XSS via query
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(`(${escaped})`, 'gi');
    const html = text.replace(regex, '<mark class="search-highlight">$1</mark>');
    return this.sanitizer.bypassSecurityTrustHtml(html);
  }
}

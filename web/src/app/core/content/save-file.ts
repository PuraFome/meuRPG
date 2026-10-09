/** Hands `text` to the browser as a file named `name` (a Blob behind an anchor); nothing is kept in Web Storage. */
export function saveTextFile(name: string, text: string, type = 'application/json'): void {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

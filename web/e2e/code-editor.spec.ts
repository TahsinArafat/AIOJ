import { test, expect } from '@playwright/test';
import { register, uniq } from './lib/api';

/**
 * The code editor is the product: a caret that jumps makes it unusable.
 *
 * `CodeEditor` used to hand Monaco a controlled `value`. React batching meant
 * that prop could still be a keystroke behind when the sync effect ran, and
 * @monaco-editor/react answers any mismatch by replacing the *whole* buffer
 * with `forceMoveMarkers` — so nearly every keystroke rewrote the file and
 * threw the caret to the last line. Typing produced the first character in the
 * right place and everything after it at the end of the document.
 */

interface MonacoEditor {
  getPosition(): { lineNumber: number; column: number } | null;
  getModel(): { getLineCount(): number } | null;
  getValue(): string;
  setPosition(position: { lineNumber: number; column: number }): void;
  setValue(value: string): void;
  focus(): void;
}

declare global {
  interface Window {
    monaco?: { editor: { getEditors(): MonacoEditor[] } };
  }
}

/** Resolves the mounted editor, failing loudly if Monaco never exposed it. */
async function editor(page: import('@playwright/test').Page) {
  const handle = await page.waitForFunction(
    () => (window.monaco?.editor.getEditors().length ?? 0) > 0,
    undefined,
    { timeout: 60_000 },
  );
  await handle.dispose();
  return {
    position: () =>
      page.evaluate(() => window.monaco!.editor.getEditors()[0].getPosition()),
    value: () => page.evaluate(() => window.monaco!.editor.getEditors()[0].getValue()),
    setPosition: (lineNumber: number, column: number) =>
      page.evaluate(
        ([line, col]) => {
          const ed = window.monaco!.editor.getEditors()[0];
          ed.setPosition({ lineNumber: line, column: col });
          ed.focus();
        },
        [lineNumber, column],
      ),
    setValue: (value: string) =>
      page.evaluate((v) => window.monaco!.editor.getEditors()[0].setValue(v), value),
  };
}

test('typing leaves the caret where the user put it', async ({ page }) => {
  const username = uniq('caret');
  const session = await register(username, `${username}@aioj.test`, 'Aioj-Sim-2026!');

  await page.addInitScript((token) => {
    localStorage.setItem('access_token', token);
  }, session.access_token);

  await page.goto('/ide');
  const ed = await editor(page);

  // Type at the very start of the document. If the caret escapes to the end,
  // the text lands on the last line instead of the first.
  await ed.setPosition(1, 1);
  await page.keyboard.type('ABCDE');

  await expect
    .poll(() => ed.position())
    .toEqual({ lineNumber: 1, column: 6 });

  expect((await ed.value()).startsWith('ABCDE')).toBe(true);

  // A second burst mid-document must stay put too: the old bug only showed up
  // once React had re-rendered with a stale value.
  await ed.setPosition(2, 1);
  await page.keyboard.type('XYZ');

  await expect
    .poll(() => ed.position())
    .toEqual({ lineNumber: 2, column: 4 });
});

test('a compile error shows the compiler diagnostics', async ({ page }) => {
  const username = uniq('ce');
  const session = await register(username, `${username}@aioj.test`, 'Aioj-Sim-2026!');

  await page.addInitScript((token) => {
    localStorage.setItem('access_token', token);
    // Force the C++ starter template, then replace it with code that cannot
    // compile.
    localStorage.setItem('aioj_ide_lang', 'cpp-gpp-64');
    localStorage.removeItem('aioj_ide_code_cpp-gpp-64');
  }, session.access_token);

  await page.goto('/ide');
  const ed = await editor(page);
  await ed.setValue('int main() { this is broken; }\n');

  await page.getByRole('button', { name: /^Run$/ }).click();

  // The panel must name the failure and carry the compiler's own message —
  // not "Compilation failed (no error output captured)", which is what the
  // sandbox returned once the compile step's stderr was left unredirected.
  await expect(page.getByText('Compilation Error', { exact: false })).toBeVisible({
    timeout: 90_000,
  });
  await expect(page.getByText(/expected .*before|error:/i).first()).toBeVisible();
});

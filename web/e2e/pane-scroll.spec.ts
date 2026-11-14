import { test, expect, type Page } from '@playwright/test';
import { createProblemWithLongStatement, loginAsSeedAdmin, uniq } from './lib/api';

/**
 * The statement pane and the editor pane on the two problem pages must scroll
 * independently.
 *
 * The regression this pins: each pane carries `overflow-y-auto`, but that only
 * does anything once the pane has a bounded height. The split container was
 * `h-full` under a `min-h-screen` shell with an auto-height page root, so
 * `h-full` resolved to `auto`, both panes stretched to whichever was taller, the
 * panes never scrolled, and the *document* did — scrolling the statement dragged
 * the editor with it.
 *
 * This lives in Playwright rather than vitest because jsdom has no layout engine:
 * `scrollHeight`/`clientHeight` are all zero there, so a unit test would pass
 * against the broken layout too.
 *
 * The problem is created with a deliberately long statement: the seeded `hello`
 * one is a single line, which never overflows a pane at any sane viewport, so a
 * test using it would pass whether or not the fix is in place.
 */

/** Both pages put their panes in these className fragments. */
const STATEMENT_PANE = 'overflow-y-auto pr-1';
const EDITOR_PANE = 'overflow-y-auto pl-1';

/** Side by side: `md` (768) for the practice page, `lg` (1024) for the contest one. */
const DESKTOP = { width: 1440, height: 800 };

function findByClass(page: Page, fragment: string) {
  // Locate each pane by the className it carries, not by "whichever element
  // overflows" — the latter would silently match some other node the day the
  // panes stop scrolling, which is exactly the failure being guarded against.
  return page.locator(`[class*="${fragment}"]`).first();
}

/** Geometry of the statement pane, the editor pane and the document. */
async function paneMetrics(page: Page) {
  return page.evaluate(
    ([stmtFrag, editorFrag]) => {
      const find = (frag: string) =>
        [...document.querySelectorAll('*')].find(
          (el) => typeof el.className === 'string' && el.className.includes(frag),
        ) as HTMLElement | undefined;

      const describe = (el?: HTMLElement) =>
        el
          ? {
              clientHeight: el.clientHeight,
              scrollHeight: el.scrollHeight,
              scrollable: el.scrollHeight > el.clientHeight + 4,
            }
          : null;

      const editor =
        document.querySelector('.monaco-editor') ??
        document.querySelector('textarea[aria-label="Solution source code"]');

      return {
        statement: describe(find(stmtFrag)),
        editorPane: describe(find(editorFrag)),
        editorTop: editor ? Math.round(editor.getBoundingClientRect().top) : null,
        docScrollable:
          document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    },
    [STATEMENT_PANE, EDITOR_PANE],
  );
}

/** Scroll the statement pane and report how far the editor moved. */
async function scrollStatementAndMeasureEditor(page: Page) {
  return page.evaluate((frag) => {
    const pane = [...document.querySelectorAll('*')].find(
      (el) => typeof el.className === 'string' && el.className.includes(frag),
    ) as HTMLElement | undefined;
    const editor =
      document.querySelector('.monaco-editor') ??
      document.querySelector('textarea[aria-label="Solution source code"]');
    if (!pane || !editor) return null;

    const before = Math.round(editor.getBoundingClientRect().top);
    pane.scrollTop = 500;
    const after = Math.round(editor.getBoundingClientRect().top);
    return {
      moved: Math.abs(after - before),
      paneScrollTop: pane.scrollTop,
      editorTopBefore: before,
      editorTopAfter: after,
    };
  }, STATEMENT_PANE);
}

/**
 * The editor must have real height and the panels below it must stay reachable.
 *
 * The regression this pins: the editor was pinned to `height="400px"`, so on a
 * tall viewport it left dead space under it (looking "shrunk" next to a
 * full-height statement), and once the right column was made to flex, the
 * scratchpad and quick links squeezed the editor instead. Both directions are
 * wrong; the editor should grow into the room left over but never collapse, and
 * the column should scroll to reach whatever does not fit.
 */
async function expectEditorUsable(page: Page) {
  const editor = await page.evaluate(() => {
    const monaco = document.querySelector('.monaco-editor');
    if (!monaco) return null;

    // The right column is the scroll container that holds the editor.
    let column: HTMLElement | null = monaco as HTMLElement;
    while (column && !(
      typeof column.className === 'string' &&
      column.className.includes('overflow-y-auto') &&
      column.className.includes('pl-1')
    )) {
      column = column.parentElement;
    }

    const quickLinks = [...document.querySelectorAll('h4')].find((h) =>
      /Quick Links/i.test(h.textContent ?? ''),
    );

    let quickLinksReachable: boolean | null = null;
    if (column && quickLinks) {
      column.scrollTop = column.scrollHeight;
      const linkRect = quickLinks.getBoundingClientRect();
      const columnRect = column.getBoundingClientRect();
      quickLinksReachable = linkRect.top >= columnRect.top - 1 && linkRect.bottom <= columnRect.bottom + 1;
    }

    return {
      height: Math.round(monaco.getBoundingClientRect().height),
      columnScrollable: column ? column.scrollHeight > column.clientHeight + 4 : null,
      quickLinksReachable,
    };
  });

  expect(editor, 'the editor must be mounted').not.toBeNull();
  // 360px floor minus Monaco's own chrome (~43px of tabs/breadcrumb).
  expect(editor!.height, 'the editor must keep a usable height').toBeGreaterThan(250);

  // Whatever does not fit must be reachable by scrolling the column, never clipped.
  if (editor!.columnScrollable) {
    expect(editor!.quickLinksReachable, 'the column must scroll to reach Quick Links').toBe(true);
  }
}

/** The assertions both problem pages share. */
async function expectIndependentPanes(page: Page) {
  const metrics = await paneMetrics(page);
  expect(metrics.statement, 'the statement pane must exist').not.toBeNull();
  expect(metrics.editorPane, 'the editor pane must exist').not.toBeNull();
  expect(
    metrics.statement!.scrollable,
    `the statement pane must overflow its own box (scrollHeight ${metrics.statement!.scrollHeight} vs clientHeight ${metrics.statement!.clientHeight})`,
  ).toBe(true);
  expect(metrics.docScrollable, 'the document must not be the scroll container').toBe(false);

  const scrolled = await scrollStatementAndMeasureEditor(page);
  expect(scrolled, 'the statement pane and editor must both be present').not.toBeNull();
  expect(scrolled!.paneScrollTop, 'the statement pane did not scroll').toBeGreaterThan(0);
  expect(scrolled!.moved, 'scrolling the statement moved the editor').toBe(0);

  await expectEditorUsable(page);
}

test('the practice problem page scrolls its panes independently', async ({ page }) => {
  await page.setViewportSize(DESKTOP);
  const admin = await loginAsSeedAdmin();
  const problem = await createProblemWithLongStatement(admin.access_token, uniq('pane'));

  await page.goto(`/problems/${problem.slug}`);
  await page.waitForLoadState('networkidle').catch(() => {});
  await expect(page.getByText('Input format').first()).toBeVisible({ timeout: 20_000 });

  // The practice page always shows the editor beside the statement at this width.
  await expect(findByClass(page, EDITOR_PANE)).toBeVisible();
  await expectIndependentPanes(page);
});

test('below the split breakpoint the document scrolls again', async ({ page }) => {
  // The panes stack on a narrow screen, so the document MUST stay scrollable —
  // bounding the shell here would clip the lower pane with no way to reach it.
  await page.setViewportSize({ width: 390, height: 844 });
  const admin = await loginAsSeedAdmin();
  const problem = await createProblemWithLongStatement(admin.access_token, uniq('pane'));

  await page.goto(`/problems/${problem.slug}`);
  await page.waitForLoadState('networkidle').catch(() => {});

  const metrics = await paneMetrics(page);
  expect(metrics.docScrollable, 'a stacked layout must still scroll the document').toBe(true);
});

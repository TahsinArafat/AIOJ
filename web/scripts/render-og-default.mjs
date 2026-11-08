/**
 * Renders web/public/og-default.png — the 1200x630 fallback card every shared
 * link uses when it has no per-entity image.
 *
 * Why a script and not a checked-in Figma export: the card is text and two
 * colours, so it can be regenerated from the repository instead of drifting out
 * of date in a binary nobody can edit. It is not part of `npm run build` — the
 * Docker build has no browser and the PNG is committed — run it by hand after
 * changing the branding:
 *
 *     cd web && node scripts/render-og-default.mjs
 *
 * Platform requirements this file satisfies: 1200x630 (1.91:1), PNG, well under
 * the 300 kB / 5 MB caps open-graph consumers apply. A card without an image is
 * what renders as a solid colour block.
 */
import { writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'

const here = dirname(fileURLToPath(import.meta.url))
const outPath = join(here, '..', 'public', 'og-default.png')

const CARD = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    html, body { width: 1200px; height: 630px; }
    body {
        position: relative;
        overflow: hidden;
        background: #070c17;
        color: #e8eefc;
        font-family: ui-sans-serif, -apple-system, "Helvetica Neue", "Segoe UI", Roboto, Arial, sans-serif;
        display: flex;
        flex-direction: column;
        justify-content: space-between;
        padding: 72px 80px;
    }
    /* Blue bloom behind the wordmark, the same #2563eb the app uses for
       --primary. Two layers: a wide wash plus a tighter hot spot. */
    .glow-a, .glow-b { position: absolute; border-radius: 50%; pointer-events: none; }
    .glow-a {
        width: 900px; height: 900px; left: -260px; top: -420px;
        background: radial-gradient(circle, rgba(37, 99, 235, 0.55) 0%, rgba(37, 99, 235, 0) 62%);
    }
    .glow-b {
        width: 620px; height: 620px; right: -180px; bottom: -300px;
        background: radial-gradient(circle, rgba(56, 189, 248, 0.32) 0%, rgba(56, 189, 248, 0) 64%);
    }
    /* Faint grid so the flat background does not read as a dead panel. */
    .grid {
        position: absolute; inset: 0; pointer-events: none;
        background-image:
            linear-gradient(to right, rgba(148, 163, 184, 0.09) 1px, transparent 1px),
            linear-gradient(to bottom, rgba(148, 163, 184, 0.09) 1px, transparent 1px);
        background-size: 60px 60px;
        mask-image: radial-gradient(ellipse 78% 70% at 22% 30%, #000 30%, transparent 78%);
        -webkit-mask-image: radial-gradient(ellipse 78% 70% at 22% 30%, #000 30%, transparent 78%);
    }
    .content { position: relative; }
    .brand { display: flex; align-items: center; gap: 18px; }
    .mark {
        width: 62px; height: 62px; border-radius: 16px;
        background: linear-gradient(150deg, #3b82f6 0%, #1d4ed8 55%, #1e3a8a 100%);
        box-shadow: 0 18px 40px -18px rgba(37, 99, 235, 0.95), inset 0 1px 0 rgba(255, 255, 255, 0.35);
        display: flex; align-items: center; justify-content: center;
        font-size: 34px; font-weight: 800; letter-spacing: -1px; color: #fff;
    }
    .brand-text { display: flex; flex-direction: column; gap: 4px; }
    .brand-name { font-size: 26px; font-weight: 700; letter-spacing: 0.02em; }
    .brand-sub { font-size: 15px; letter-spacing: 0.22em; text-transform: uppercase; color: #7d93b8; }

    h1 {
        font-size: 108px;
        line-height: 0.96;
        letter-spacing: -0.035em;
        font-weight: 800;
        background: linear-gradient(94deg, #ffffff 12%, #cfe0ff 52%, #7fb2ff 100%);
        -webkit-background-clip: text;
        background-clip: text;
        color: transparent;
        margin-top: 46px;
    }
    .accent { color: #3b82f6; -webkit-text-fill-color: #3b82f6; }
    .tagline {
        margin-top: 22px;
        font-size: 27px;
        line-height: 1.42;
        color: #9fb2ce;
        max-width: 800px;
    }
    .footer {
        position: relative;
        display: flex; align-items: flex-end; justify-content: space-between;
        border-top: 1px solid rgba(148, 163, 184, 0.22);
        padding-top: 26px;
    }
    .chips { display: flex; gap: 12px; }
    .chip {
        font-size: 16px; letter-spacing: 0.06em; text-transform: uppercase;
        color: #b9c9e4;
        border: 1px solid rgba(148, 163, 184, 0.28);
        border-radius: 999px;
        padding: 8px 16px;
        background: rgba(148, 163, 184, 0.07);
    }
    .domain { font-size: 24px; font-weight: 600; letter-spacing: 0.01em; color: #dbe6fb; }
</style>
</head>
<body>
    <div class="glow-a"></div>
    <div class="glow-b"></div>
    <div class="grid"></div>
    <div class="content">
        <div class="brand">
            <div class="mark">A</div>
            <div class="brand-text">
                <div class="brand-name">AIOJ</div>
                <div class="brand-sub">Online Judge</div>
            </div>
        </div>
        <h1>Compete.<br />Solve.<span class="accent"> Rank.</span></h1>
        <div class="tagline">
            Problems, contests and instant judging — with a rating that tracks every round you enter.
        </div>
    </div>
    <div class="footer">
        <div class="chips">
            <div class="chip">Problems</div>
            <div class="chip">Contests</div>
            <div class="chip">Rankings</div>
        </div>
        <div class="domain">aioj0.cyou</div>
    </div>
</body>
</html>`

const browser = await chromium.launch()
try {
    const page = await browser.newPage({ viewport: { width: 1200, height: 630 }, deviceScaleFactor: 1 })
    await page.setContent(CARD, { waitUntil: 'load' })
    // Fail loudly if the copy has grown enough to leave the frame: a card that
    // clips its tagline looks broken in every share, and the file is only ever
    // regenerated by hand, so nothing else would catch it.
    //
    // Measured per element rather than via the document scroll size: the two
    // decorative glows are intentionally positioned past the edges and clipped
    // by `overflow: hidden`, which inflates the document box.
    const layout = await page.evaluate(() => {
        const problems = []
        const boxes = ['.content', '.brand', 'h1', '.tagline', '.footer', '.chips', '.domain']
        for (const selector of boxes) {
            const el = document.querySelector(selector)
            if (!el) {
                problems.push(`${selector} is missing`)
                continue
            }
            const r = el.getBoundingClientRect()
            if (r.left < -0.5 || r.top < -0.5 || r.right > 1200.5 || r.bottom > 630.5) {
                problems.push(
                    `${selector} is out of frame at ${Math.round(r.left)},${Math.round(r.top)} ` +
                        `(${Math.round(r.width)}x${Math.round(r.height)})`
                )
            }
            if (el.scrollWidth > el.clientWidth + 1) {
                problems.push(`${selector} overflows horizontally`)
            }
        }
        return problems
    })
    if (layout.length > 0) {
        throw new Error(`card layout broke: ${layout.join('; ')}`)
    }
    // Screenshot the viewport, not the full page: the card must be exactly
    // 1200x630 or platforms crop or letterbox it.
    const png = await page.screenshot({ type: 'png', fullPage: false })
    await writeFile(outPath, png)
    console.log(`wrote ${outPath} (${(png.length / 1024).toFixed(1)} kB)`)
    if (png.length > 300 * 1024) {
        console.warn('warning: over 300 kB — some platforms truncate or skip the preview')
    }
} finally {
    await browser.close()
}

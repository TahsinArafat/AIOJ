/**
 * Behaviour for the server-rendered contest booklet.
 *
 * This lives in a file rather than inline in the booklet HTML because the site
 * CSP is `script-src 'self' blob:` — that forbids inline <script> blocks AND
 * inline event handlers (onload=, onclick=). The first attempt at this fix put
 * the render call in an inline script, so the browser blocked it and the booklet
 * still showed literal $...$ with no way to print.
 *
 * Everything here is progressive enhancement: the booklet is complete, printable
 * HTML without it. Missing KaTeX degrades to readable source text, not a blank page.
 */
;(function () {
  'use strict'

  function renderMath() {
    if (typeof window.renderMathInElement !== 'function') {
      // KaTeX failed to load. Leave the source text alone rather than mangling it.
      return
    }
    window.renderMathInElement(document.body, {
      delimiters: [
        { left: '$$', right: '$$', display: true },
        { left: '$', right: '$', display: false },
        { left: '\\[', right: '\\]', display: true },
        { left: '\\(', right: '\\)', display: false }
      ],
      // A single bad expression must not blank the rest of the statement.
      throwOnError: false
    })
    document.documentElement.classList.add('math-ready')
  }

  function wirePrint() {
    var btn = document.getElementById('aioj-print')
    if (btn) {
      btn.addEventListener('click', function () {
        window.print()
      })
    }
  }

  // KaTeX is loaded with `defer`, so by DOMContentLoaded it has executed. If it
  // somehow has not, the typeset pass is skipped rather than thrown.
  function init() {
    renderMath()
    wirePrint()
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init)
  } else {
    init()
  }
})()

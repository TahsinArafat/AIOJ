import { describe, expect, test } from 'vitest'
import { htmlToMarkdown, markdownToHtml } from './markdown'

// The statement editor round-trips everything the author writes through
// htmlToMarkdown on every keystroke. Anything this pair does not preserve is
// destroyed the first time someone saves, so each case here is a data-loss
// regression rather than a formatting preference.
describe('editor round-trip', () => {
    test('keeps an uploaded image', () => {
        const md = '![Figure 1](/media/abc.png)'
        expect(htmlToMarkdown(markdownToHtml(md)).trim()).toBe(md)
    })

    test('keeps an image title', () => {
        const md = '![Alt](/media/x.png "A title")'
        expect(htmlToMarkdown(markdownToHtml(md)).trim()).toBe(md)
    })

    test('keeps a pipe table', () => {
        const back = htmlToMarkdown(markdownToHtml('| a | b |\n| --- | --- |\n| 1 | 2 |')).trim()
        expect(back).toBe('| a | b |\n| --- | --- |\n| 1 | 2 |')
    })

    test('keeps math inside a table cell', () => {
        // Math is an empty <math data-latex> node: a textContent-based table
        // rule silently blanks every formula in the table.
        const back = htmlToMarkdown(markdownToHtml('| $n$ | bound |\n| --- | --- |\n| 1 | $\\le 10$ |')).trim()
        expect(back).toContain('$n$')
        expect(back).toContain('$\\le 10$')
    })

    test('escapes a pipe inside a cell so it stays one column', () => {
        const back = htmlToMarkdown(markdownToHtml('| a | b |\n| --- | --- |\n| x\\|y | z |')).trim()
        expect(back).toContain('x\\|y')
    })

    test('keeps inline and display math', () => {
        expect(htmlToMarkdown(markdownToHtml('$1 \\le n$')).trim()).toBe('$1 \\le n$')
        expect(htmlToMarkdown(markdownToHtml('$$x^2$$')).trim()).toBe('$$x^2$$')
    })
})

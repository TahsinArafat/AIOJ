import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { expect, test } from 'vitest'
import MathExtension from './MathExtension'
import { htmlToMarkdown, markdownToHtml } from '../../lib/markdown'

const bounds = '$1 \\le n \\le 2\\cdot10^5$\n$1 \\le q \\le 2\\cdot10^5$\n$998244353$ is prime'

/** Seeded the way the editor seeds itself, then saved the way it saves. */
const save = (markdown: string) => {
  const editor = new Editor({
    element: document.createElement('div'),
    extensions: [StarterKit.configure({ codeBlock: false }), MathExtension],
    content: markdownToHtml(markdown),
  })
  return htmlToMarkdown(editor.getHTML())
}

// The whole bug: opening a stored statement and saving it again has to be a
// no-op. It was not — newlines collapsed and every `\le` came back as `\\le`,
// which KaTeX renders as "l e".
test('a stored statement survives open and save unchanged', () => {
  expect(save(bounds)).toBe(bounds)
})

test('keeps paragraph breaks apart from line breaks', () => {
  const statement = 'first line\nsecond line\n\nnext paragraph'

  expect(save(statement)).toBe(statement)
})

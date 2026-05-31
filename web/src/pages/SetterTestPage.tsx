import { useState } from 'react'
import VisualEditor from '../components/VisualEditor'

const SAMPLE_MARKDOWN = `# Weird Algorithm

Consider an algorithm that takes as input a positive integer $n$. If $n$ is even, the algorithm divides it by two, and if $n$ is odd, the algorithm multiplies it by three and adds one. The algorithm repeats this, until $n$ is one. For example, the sequence for $n=3$ is as follows:

$$3 \\rightarrow 10 \\rightarrow 5 \\rightarrow 16 \\rightarrow 8 \\rightarrow 4 \\rightarrow 2 \\rightarrow 1$$

Your task is to simulate the execution of the algorithm for a given value of $n$.

## Input

The only input line contains an integer $n$.

## Output

Print a line that contains all values of $n$ during the algorithm.

## Constraints

- $1 \\le n \\le 10^6$

## Example

**Input:**
\`\`\`
3
\`\`\`

**Output:**
\`\`\`
3 10 5 16 8 4 2 1
\`\`\`
`

export default function SetterTestPage() {
  const [markdown, setMarkdown] = useState(SAMPLE_MARKDOWN)
  const [showMarkdown, setShowMarkdown] = useState(false)

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-4xl mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold">Visual Editor Test Page</h1>
          <div className="flex gap-2">
            <button
              onClick={() => setShowMarkdown(!showMarkdown)}
              className="px-4 py-2 text-sm border rounded hover:bg-gray-100"
            >
              {showMarkdown ? 'Hide' : 'Show'} Markdown
            </button>
            <button
              onClick={() => {
                navigator.clipboard.writeText(markdown)
                alert('Markdown copied!')
              }}
              className="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700"
            >
              Copy Markdown
            </button>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6 mb-6">
          <h2 className="text-lg font-semibold mb-4">Editor</h2>
          <VisualEditor
            content={markdown}
            onChange={setMarkdown}
            placeholder="Start writing your problem statement..."
          />
        </div>

        {showMarkdown && (
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold mb-4">Generated Markdown</h2>
            <pre className="bg-gray-100 rounded p-4 text-sm font-mono overflow-auto max-h-[400px]">
              {markdown}
            </pre>
          </div>
        )}
      </div>
    </div>
  )
}

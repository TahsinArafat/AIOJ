import type { ProblemFormState } from '../../types/problem-workspace'
import CodeEditor from '../CodeEditor'

interface CheckerTabProps {
  formState: ProblemFormState
  saving: boolean
  onUpdate: <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => void
  onSave: () => void
}

export default function CheckerTab({ formState, saving, onUpdate, onSave }: CheckerTabProps) {
  const precisionFloatPreset = `#include <iostream>
#include <fstream>
#include <cmath>

using namespace std;

int main(int argc, char* argv[]) {
    if (argc < 4) {
        cerr << "Usage: spj <input> <user> <answer>" << endl;
        return 2;
    }
    
    ifstream fin(argv[1]);
    ifstream fuser(argv[2]);
    ifstream fans(argv[3]);
    
    double userVal, ansVal;
    if (!(fuser >> userVal)) {
        cerr << "Wrong Answer: Failed to read user float token" << endl;
        return 1;
    }
    if (!(fans >> ansVal)) {
        cerr << "System Error: Failed to read expected answer float token" << endl;
        return 2;
    }
    
    double diff = abs(userVal - ansVal);
    if (diff > 1e-9 && diff / max(1.0, abs(ansVal)) > 1e-9) {
        cerr << "Wrong Answer: Difference too large! Expected " << ansVal << ", got " << userVal << " (diff: " << diff << ")" << endl;
        return 1;
    }
    
    cout << "OK: Floats match within 1e-9" << endl;
    return 0;
}`

  const arrayGraphPreset = `#include <iostream>
#include <fstream>
#include <vector>

using namespace std;

int main(int argc, char* argv[]) {
    ifstream fin(argv[1]);   // Input case
    ifstream fuser(argv[2]); // User stdout
    ifstream fans(argv[3]);  // Expected output
    
    int n;
    fin >> n;
    
    vector<int> userArr(n);
    for (int i = 0; i < n; i++) {
        if (!(fuser >> userArr[i])) {
            cerr << "Wrong Answer: Insufficient numbers of tokens" << endl;
            return 1;
        }
    }
    
    for (int i = 1; i < n; i++) {
        if (userArr[i] < userArr[i-1]) {
            cerr << "Wrong Answer: Array is not sorted at index " << i << endl;
            return 1;
        }
    }
    
    cout << "OK: Sorted output verified" << endl;
    return 0;
}`

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Checker & Special Judge Configuration</h2>

      <div>
        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Checker Method</label>
        <select
          value={formState.checkerType}
          onChange={e => onUpdate('checkerType', e.target.value)}
          className="border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white text-gray-800"
        >
          <option value="exact">Exact Bytes Match (Standard)</option>
          <option value="lines">Lines Differences (Ignore Trailing Spaces)</option>
          <option value="float">Float Tolerance Precision</option>
          <option value="float_absolute">Floating Point (Absolute Epsilon)</option>
          <option value="float_relative">Floating Point (Relative Epsilon)</option>
          <option value="custom">Custom Special Judge (SPJ)</option>
        </select>
      </div>

      {(formState.checkerType === 'float' || formState.checkerType === 'float_absolute' || formState.checkerType === 'float_relative') && (
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Float Epsilon (Precision Tolerance)</label>
          <input
            type="number"
            step="any"
            value={formState.floatEpsilon}
            onChange={e => onUpdate('floatEpsilon', Number(e.target.value))}
            className="border border-gray-300 rounded px-3 py-1.5 text-sm w-48 font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white text-gray-800"
          />
        </div>
      )}

      {formState.checkerType === 'custom' && (
        <div className="space-y-4">
          <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 text-gray-300 shadow-md space-y-3">
            <div className="flex items-center space-x-2 text-yellow-400">
              <span className="w-2.5 h-2.5 rounded-full bg-yellow-400 animate-pulse" />
              <p className="font-bold text-xs uppercase tracking-wider">Special Judge (SPJ) Sandbox Environment Protocol</p>
            </div>
            <p className="text-xs leading-relaxed text-gray-400">
              Your compiled C++ checker binary runs inside a secure isolated sandbox with execution parameters:
            </p>
            <div className="bg-black/40 border border-gray-800 rounded p-2.5 font-mono text-[11px] text-emerald-400 select-all">
              ./spj input.txt user.txt answer.txt
            </div>
            <div className="grid grid-cols-2 gap-4 pt-2 text-[11px]">
              <div className="space-y-1.5">
                <span className="font-semibold block text-gray-400 uppercase tracking-wider text-[10px]">Compilation Outputs</span>
                <ul className="list-disc pl-4 space-y-0.5 text-gray-500">
                  <li>Standard streams are fully captured</li>
                  <li>Errors output on <span className="font-semibold text-gray-400">stderr</span> as details</li>
                </ul>
              </div>
              <div className="space-y-1.5">
                <span className="font-semibold block text-gray-400 uppercase tracking-wider text-[10px]">Return Status Codes</span>
                <ul className="list-disc pl-4 space-y-0.5 text-gray-500">
                  <li><span className="font-semibold text-emerald-400">exit status 0</span>: Accepted (AC)</li>
                  <li><span className="font-semibold text-rose-400">exit status 1</span>: Wrong Answer (WA)</li>
                </ul>
              </div>
            </div>
          </div>

          <div className="bg-white border border-gray-200 rounded-lg p-4 shadow-sm space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <div>
                <span className="text-xs text-gray-600 font-bold block uppercase tracking-wider">Advanced SPJ Presets</span>
                <span className="text-[11px] text-gray-400">Populate boilerplate templates directly into the editor</span>
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => onUpdate('spjSourceCode', precisionFloatPreset)}
                  className="bg-gray-50 border border-gray-200 text-gray-700 px-3 py-1.5 rounded-md text-xs hover:bg-gray-100 hover:text-black transition-colors font-semibold cursor-pointer"
                >
                  Precision Float Presets
                </button>
                <button
                  type="button"
                  onClick={() => onUpdate('spjSourceCode', arrayGraphPreset)}
                  className="bg-gray-50 border border-gray-200 text-gray-700 px-3 py-1.5 rounded-md text-xs hover:bg-gray-100 hover:text-black transition-colors font-semibold cursor-pointer"
                >
                  Array/Graph Presets
                </button>
              </div>
            </div>

            <div>
              <label className="block text-[10px] font-bold text-gray-400 uppercase tracking-wider mb-1">SPJ Sandbox Language</label>
              <select
                value={formState.spjLanguage}
                onChange={e => onUpdate('spjLanguage', e.target.value)}
                className="border border-gray-300 rounded px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white text-gray-800"
              >
                <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
              </select>
            </div>

            <div>
              <label className="block text-[10px] font-bold text-gray-400 uppercase tracking-wider mb-1">SPJ Source Code Editor</label>
              <div className="border border-gray-200 rounded overflow-hidden shadow-inner bg-gray-50">
                <CodeEditor
                  language={formState.spjLanguage}
                  value={formState.spjSourceCode}
                  onChange={(val: string) => onUpdate('spjSourceCode', val)}
                  height="350px"
                />
              </div>
            </div>
          </div>
        </div>
      )}

      <hr className="border-gray-200" />

      <div className="space-y-3">
        <h3 className="text-sm font-bold text-gray-700 uppercase tracking-wider">Interactive Problem</h3>
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="interactive"
            checked={formState.interactive}
            onChange={e => onUpdate('interactive', e.target.checked)}
            className="h-4 w-4 border-gray-300 rounded text-blue-600 focus:ring-blue-500 bg-white"
          />
          <label htmlFor="interactive" className="text-sm font-medium text-gray-700">
            Interactive Problem (Judge communicates via stdin/stdout)
          </label>
        </div>

        {formState.interactive && (
          <div className="space-y-3 pl-6 border-l-2 border-blue-200">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Language</label>
              <select
                value={formState.interactorLanguage}
                onChange={e => onUpdate('interactorLanguage', e.target.value)}
                className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white text-gray-800"
              >
                <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
                <option value="python">Python 3</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Source Code</label>
              <div className="border border-gray-200 rounded overflow-hidden shadow-inner bg-gray-50">
                <CodeEditor
                  language={formState.interactorLanguage}
                  value={formState.interactorSourceCode}
                  onChange={(val: string) => onUpdate('interactorSourceCode', val)}
                  height="200px"
                />
              </div>
            </div>
          </div>
        )}
      </div>

      <button
        onClick={onSave}
        disabled={saving}
        className="bg-blue-600 text-white px-5 py-2 rounded text-sm font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
      >
        {saving ? 'Saving...' : 'Save Checker Configuration'}
      </button>
    </div>
  )
}

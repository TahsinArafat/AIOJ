package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every language in lang/*.yaml names a compiler or runtime by absolute path.
// Nothing verified those paths existed, which is how four languages shipped
// broken at once:
//
//   - cpp-clang pointed at /usr/bin/clang++, but Dockerfile.judge never
//     installed clang;
//   - javascript pointed at /usr/bin/js, which no Debian package provides (the
//     nodejs package ships /usr/bin/node and /usr/bin/nodejs);
//   - the *-32 languages needed multilib, absent from the image;
//   - java's {{dir}} resolved to /box, which is isolate's convention — judge
//     runs go-judge, whose working directory is /w.
//
// The image's package list is the only source of truth for what is installed,
// and it lives outside this repo's compiler package, so this test reads the
// Dockerfile and checks each declared binary against it.
//
// It is a static check, not a substitute for a live sandbox smoke test: it
// catches "the config names something the image never installs", which is the
// failure that actually happened. It cannot catch a package installing a
// binary under a different name at build time.

// langDir is ../../../lang relative to internal/judge/compiler.
const langDir = "../../../lang"

// binaryToPackage maps a binary's absolute path to the package that provides
// it, as installed by Dockerfile.judge. Keeping the mapping explicit means a
// new language config cannot be added without deciding where its binary comes
// from.
var binaryToPackage = map[string]string{
	"/usr/bin/gcc":     "gcc",
	"/usr/bin/g++":     "g++",
	"/usr/bin/clang":   "clang",
	"/usr/bin/clang++": "clang",
	"/usr/bin/rustc":   "rustc",
	"/usr/bin/javac":   "openjdk-21-jdk-headless",
	"/usr/bin/java":    "openjdk-21-jdk-headless",
	"/usr/bin/python3": "python3",
	"/usr/bin/pypy3":   "pypy3",
	"/usr/bin/node":    "nodejs",
	"/usr/bin/nodejs":  "nodejs",
	"/usr/bin/mcs":     "mono-mcs",
	"/usr/bin/mono":    "mono-runtime",
}

func TestEveryLanguageBinaryIsInstalledByTheJudgeImage(t *testing.T) {
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "..", "Dockerfile.judge"))
	if err != nil {
		t.Fatalf("read Dockerfile.judge: %v", err)
	}
	installed := installedPackages(string(dockerfile))

	langs, err := LoadLanguages(langDir)
	if err != nil {
		t.Fatalf("load languages: %v", err)
	}
	if len(langs) == 0 {
		t.Fatal("no languages loaded; the test would pass vacuously")
	}

	keys := make([]string, 0, len(langs))
	for k := range langs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		cfg := langs[key]
		for _, binary := range referencedBinaries(cfg) {
			pkg, known := binaryToPackage[binary]
			if !known {
				t.Errorf("%s uses %s, which this test does not know how to place. "+
					"Add it to binaryToPackage and install it in Dockerfile.judge.", key, binary)
				continue
			}
			if !installed[pkg] {
				t.Errorf("%s uses %s from package %q, but Dockerfile.judge does not "+
					"install it — every submission in this language fails at run time",
					key, binary, pkg)
			}
		}
	}
}

// TestNoLanguageUsesTheIsolateWorkingDirectory pins the /box trap: go-judge
// starts commands in /w, so any /box path in a language config is dead.
func TestNoLanguageUsesTheIsolateWorkingDirectory(t *testing.T) {
	langs, err := LoadLanguages(langDir)
	if err != nil {
		t.Fatalf("load languages: %v", err)
	}
	for key, cfg := range langs {
		for _, field := range []string{cfg.CompileCmd, cfg.Runtime} {
			if strings.Contains(field, "/box") {
				t.Errorf("%s references /box, which does not exist in go-judge "+
					"(its sandbox working directory is %s): %q", key, SandboxWorkDir, field)
			}
		}
	}
}

// referencedBinaries pulls the absolute executable paths out of a config's
// compile and runtime commands.
func referencedBinaries(cfg *LangConfig) []string {
	var out []string
	seen := make(map[string]bool)
	for _, field := range []string{cfg.CompileCmd, cfg.Runtime} {
		for _, tok := range strings.Fields(field) {
			// Only a leading absolute path is the program; later tokens are its
			// arguments (e.g. `-o {{exe}}`), which are not binaries.
			if !strings.HasPrefix(tok, "/usr/") {
				continue
			}
			if seen[tok] {
				continue
			}
			seen[tok] = true
			out = append(out, tok)
		}
	}
	return out
}

// installedPackages extracts package names from the apt-get install line.
//
// Comments are stripped first: the Dockerfile explains *why* clang and multilib
// are installed, and those explanations name the very packages under test. A
// parser that read comment words as package names would keep passing after the
// real install line was deleted — exactly the failure this test exists to catch.
func installedPackages(dockerfile string) map[string]bool {
	installed := make(map[string]bool)
	for _, line := range strings.Split(dockerfile, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		for _, tok := range strings.Fields(line) {
			tok = strings.TrimSuffix(tok, "\\")
			if tok == "" {
				continue
			}
			installed[tok] = true
		}
	}
	return installed
}

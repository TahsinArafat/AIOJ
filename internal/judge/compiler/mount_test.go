package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The 32-bit languages failed at link time with:
//
//	/usr/bin/ld: cannot find /lib32/libc.so.6: No such file or directory
//	/usr/bin/ld: cannot find /lib/ld-linux.so.2: No such file or directory
//
// while their 64-bit siblings compiled fine. The multilib packages were
// installed; what was missing was /lib32 in the go-judge sandbox mount table.
// go-judge's baked mount.yaml binds /bin /lib /lib64 /usr but not /lib32, and
// because a host-side `gcc -m32` succeeds against the image's real filesystem,
// nothing in the repo noticed.
//
// This test pins the contract the fix depends on: every *-32 language needs
// /lib32 inside the sandbox, so the mount file we ship must bind it.

// sandboxMountConf is deploy/judge/mount.yaml, relative to this package.
const sandboxMountConf = "../../../deploy/judge/mount.yaml"

type mountEntry struct {
	Type   string `yaml:"type"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type mountConf struct {
	Mount []mountEntry `yaml:"mount"`
}

func loadSandboxMounts(t *testing.T) mountConf {
	t.Helper()
	data, err := os.ReadFile(sandboxMountConf)
	if err != nil {
		t.Fatalf("read %s: %v", sandboxMountConf, err)
	}
	var conf mountConf
	if err := yaml.Unmarshal(data, &conf); err != nil {
		t.Fatalf("%s is not valid YAML: %v", sandboxMountConf, err)
	}
	if len(conf.Mount) == 0 {
		t.Fatalf("%s declares no mounts; the test would pass vacuously", sandboxMountConf)
	}
	return conf
}

// TestSandboxMountsExposeLib32 is the regression test for the 32-bit link
// failure. Without this bind the linker cannot resolve libc.so.6 and every
// *-32 submission is a Compilation Error.
func TestSandboxMountsExposeLib32(t *testing.T) {
	conf := loadSandboxMounts(t)

	var found bool
	for _, m := range conf.Mount {
		if m.Type == "bind" && m.Target == "/lib32" {
			found = true
			if m.Source != "/lib32" {
				t.Errorf("/lib32 is bound from %q; the 32-bit libc lives at /lib32 in the judge image", m.Source)
			}
		}
	}
	if !found {
		t.Fatalf("%s does not bind /lib32. Every C/C++ -m32 submission fails to link "+
			"with \"cannot find /lib32/libc.so.6\" — add the bind described at the top of that file", sandboxMountConf)
	}
}

// TestEvery32BitLanguageIsCoveredByTheMount ties the language configs to the
// mount table. A new -m32 language added without re-checking the sandbox would
// otherwise ship broken in exactly the way this fix was written for.
func TestEvery32BitLanguageIsCoveredByTheMount(t *testing.T) {
	langs, err := LoadLanguages(langDir)
	if err != nil {
		t.Fatalf("load languages: %v", err)
	}

	var bit32 []string
	var mounted32 bool
	for _, m := range loadSandboxMounts(t).Mount {
		if m.Target == "/lib32" {
			mounted32 = true
		}
	}

	for key, cfg := range langs {
		if !strings.Contains(cfg.CompileCmd, "-m32") {
			continue
		}
		bit32 = append(bit32, key)
		if !mounted32 {
			t.Errorf("%s compiles with -m32 but the sandbox does not mount /lib32, "+
				"so the link step cannot find libc.so.6", key)
		}
	}

	// If the configs stop using -m32 entirely this test proves nothing, and the
	// languages should be dropped from lang/ rather than left to rot silently.
	if len(bit32) == 0 {
		t.Skip("no -m32 language configs; nothing to check")
	}
	t.Logf("32-bit languages relying on the /lib32 mount: %s", strings.Join(bit32, ", "))
}

// TestSandboxMountsKeepTheVendorEntries guards the wholesale replacement.
//
// deploy/judge/mount.yaml is copied over go-judge's own /opt/mount.yaml, so a
// mount missing here is a mount missing from every judgment. Dropping
// /etc/java-21-openjdk, for example, breaks Java with no error at build time.
// This list mirrors the vendor file for the entries that carry a real
// dependency; the optional /dev binds are excluded because losing one degrades
// a program rather than breaking a language.
func TestSandboxMountsKeepTheVendorEntries(t *testing.T) {
	targets := make(map[string]bool)
	for _, m := range loadSandboxMounts(t).Mount {
		targets[m.Target] = true
	}

	required := []string{
		"/bin",
		"/lib",
		"/lib64",
		"/usr",
		"/etc/ld.so.cache",
		"/etc/alternatives",
		"/etc/java-21-openjdk", // Java: JVM needs its config tree
		"/etc/mono",            // C#: Mono needs its config tree
		"/dev/null",            // most toolchains
		"/w",                   // SandboxWorkDir — nothing runs without it
		"/tmp",
	}
	for _, target := range required {
		if !targets[target] {
			t.Errorf("%s does not mount %s. It replaces go-judge's vendor mount.yaml "+
				"outright, so an entry omitted here is absent from every judgment", sandboxMountConf, target)
		}
	}
}

// TestSandboxWorkDirMatchesTheConfig pins the mount target against the constant
// every compile command depends on: a mismatch sends {{dir}} somewhere unwritable.
func TestSandboxWorkDirMatchesTheConfig(t *testing.T) {
	var workDir string
	data, err := os.ReadFile(sandboxMountConf)
	if err != nil {
		t.Fatalf("read %s: %v", sandboxMountConf, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "workDir:"); ok {
			workDir = strings.TrimSpace(rest)
		}
	}
	if workDir != SandboxWorkDir {
		t.Fatalf("%s sets workDir: %q, but compile commands address %q (SandboxWorkDir). "+
			"Java's {{dir}} output and every absolute-path artifact depend on these agreeing",
			filepath.Base(sandboxMountConf), workDir, SandboxWorkDir)
	}
}

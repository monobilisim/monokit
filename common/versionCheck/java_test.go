package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJavaVersion(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "Temurin 25 (elk1.ist2.mono.biz)",
			output: `openjdk version "25.0.1" 2025-10-21 LTS
OpenJDK Runtime Environment Temurin-25.0.1+8 (build 25.0.1+8-LTS)
OpenJDK 64-Bit Server VM Temurin-25.0.1+8 (build 25.0.1+8-LTS, mixed mode, sharing)`,
			want: "25.0.1",
		},
		{
			name: "Debian OpenJDK 21",
			output: `openjdk version "21.0.13" 2026-10-20
OpenJDK Runtime Environment (build 21.0.13+9-Debian-1deb13u2)
OpenJDK 64-Bit Server VM (build 21.0.13+9-Debian-1deb13u2, mixed mode, sharing)`,
			want: "21.0.13",
		},
		{
			name: "Debian OpenJDK 21 security update",
			output: `openjdk version "21.0.13.1" 2026-11-04
OpenJDK Runtime Environment (build 21.0.13.1+2-Debian-1deb13u1)`,
			want: "21.0.13.1",
		},
		{
			name: "OpenJDK 8",
			output: `openjdk version "1.8.0_432"
OpenJDK Runtime Environment (build 1.8.0_432-b06)`,
			want: "1.8.0_432",
		},
		{
			name: "Oracle JDK 8",
			output: `java version "1.8.0_391"
Java(TM) SE Runtime Environment (build 1.8.0_391-b13)`,
			want: "1.8.0_391",
		},
		{
			name: "Oracle JDK 21",
			output: `java version "21.0.5" 2024-10-15 LTS
Java(TM) SE Runtime Environment (build 21.0.5+9-LTS-239)`,
			want: "21.0.5",
		},
		{
			name: "Corretto 17",
			output: `openjdk version "17.0.13" 2024-10-15 LTS
OpenJDK Runtime Environment Corretto-17.0.13.11.1 (build 17.0.13+11-LTS)`,
			want: "17.0.13",
		},
		{
			name: "Zulu 11",
			output: `openjdk version "11.0.25" 2024-10-15 LTS
OpenJDK Runtime Environment Zulu11.76+21-CA (build 11.0.25+9-LTS)`,
			want: "11.0.25",
		},
		{
			name: "GraalVM 21",
			output: `java version "21.0.5" 2024-10-15
Java(TM) SE Runtime Environment Oracle GraalVM 21.0.5+9.1 (build 21.0.5+9-LTS-jvmci-23.1-b48)`,
			want: "21.0.5",
		},
		{
			name:   "Non-matching output",
			output: "bash: java: command not found",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseJavaVersion(tt.output); got != tt.want {
				t.Errorf("parseJavaVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJavaStoreKey(t *testing.T) {
	got := javaStoreKey("/usr/share/opensearch/jdk/bin/java")
	want := "java-usr-share-opensearch-jdk-bin-java"
	if got != want {
		t.Fatalf("javaStoreKey() = %q, want %q", got, want)
	}
}

func TestEOLLookupAndNormalizeJava(t *testing.T) {
	slug, key, ok := eolLookup("Java")
	if !ok || slug != "eclipse-temurin" || key != "eclipse-temurin_eol" {
		t.Fatalf("eolLookup(Java) = (%q, %q, %v), want (eclipse-temurin, eclipse-temurin_eol, true)", slug, key, ok)
	}

	slug2, key2, ok2 := eolLookup("Java (/usr/share/opensearch/jdk/bin/java)")
	if !ok2 || slug2 != "eclipse-temurin" || key2 != "java-usr-share-opensearch-jdk-bin-java_eol" {
		t.Fatalf("eolLookup(Java path) = (%q, %q, %v), want (eclipse-temurin, java-usr-share-opensearch-jdk-bin-java_eol, true)", slug2, key2, ok2)
	}

	if key == key2 {
		t.Fatalf("expected distinct state keys for primary and path-based Java entries, both got %q", key)
	}

	nLatest := normalizeVersionForEOL("eclipse-temurin", "25.0.5.1+2")
	if cmp := compareVersions(nLatest, normalizeVersionForEOL("eclipse-temurin", "25.0.5.1")); cmp != 0 {
		t.Errorf("expected 25.0.5.1+2 and 25.0.5.1 to compare equal after normalization, got %d", cmp)
	}

	if cmp := compareVersions(nLatest, normalizeVersionForEOL("eclipse-temurin", "25.0.1")); cmp <= 0 {
		t.Errorf("expected 25.0.5.1+2 > 25.0.1 after normalization, got %d", cmp)
	}

	if got := normalizeVersionForEOL("eclipse-temurin", "1.8.0_432"); got != "8.0.432" {
		t.Errorf("normalizeVersionForEOL(1.8.0_432) = %q, want 8.0.432", got)
	}
}

func TestResolveExecutableJavaDeduplication(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "java-real")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	link1 := filepath.Join(dir, "java-link1")
	link2 := filepath.Join(dir, "java-link2")
	if err := os.Symlink(target, link1); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatal(err)
	}

	rTarget, ok := resolveExecutableJava(target)
	rLink1, ok1 := resolveExecutableJava(link1)
	rLink2, ok2 := resolveExecutableJava(link2)
	if !ok || !ok1 || !ok2 {
		t.Fatalf("expected all executable paths to resolve: %v %v %v", ok, ok1, ok2)
	}
	if rTarget != rLink1 || rTarget != rLink2 {
		t.Fatalf("expected identical canonical paths, got %q, %q, %q", rTarget, rLink1, rLink2)
	}

	nonExec := filepath.Join(dir, "java-nonexec")
	if err := os.WriteFile(nonExec, []byte("not exec"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolveExecutableJava(nonExec); ok {
		t.Fatalf("expected non-executable file to be rejected")
	}
}

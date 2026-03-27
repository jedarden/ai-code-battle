package validator

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

// CheckSyntax validates the syntax of code for the given language.
// Returns nil when the code is syntactically valid.
//
// For Go it uses the stdlib go/parser (no subprocess).  For other
// languages it shells out to the language's own syntax-checker binary;
// if the binary is not installed it falls back to a brace-balance check.
func CheckSyntax(ctx context.Context, code, language string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch language {
	case "go":
		return checkGoSyntax(code)
	case "python":
		return checkWithTempFile(ctx, code, "bot.py",
			func(path string) *exec.Cmd {
				return exec.CommandContext(ctx, "python3", "-m", "py_compile", path)
			})
	case "rust":
		return checkRustSyntax(ctx, code)
	case "typescript":
		return checkTSSyntax(ctx, code)
	case "java":
		return checkJavaSyntax(ctx, code)
	case "php":
		return checkWithTempFile(ctx, code, "bot.php",
			func(path string) *exec.Cmd {
				return exec.CommandContext(ctx, "php", "-l", path)
			})
	default:
		return fmt.Errorf("unsupported language: %s", language)
	}
}

// checkGoSyntax uses the stdlib go/parser to validate Go source.
// This is fast, dependency-free, and catches all parse errors.
func checkGoSyntax(code string) error {
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "bot.go", code, 0); err != nil {
		return fmt.Errorf("go syntax: %w", err)
	}
	return nil
}

// checkRustSyntax tries rustfmt --check first, then falls back to brace balance.
func checkRustSyntax(ctx context.Context, code string) error {
	if _, err := exec.LookPath("rustfmt"); err == nil {
		return checkWithTempFile(ctx, code, "bot.rs",
			func(path string) *exec.Cmd {
				return exec.CommandContext(ctx, "rustfmt", "--check", path)
			})
	}
	return checkBraceBalance(code, "rust")
}

// checkTSSyntax runs tsc --noEmit when available, then falls back to brace balance.
func checkTSSyntax(ctx context.Context, code string) error {
	if _, err := exec.LookPath("tsc"); err != nil {
		return checkBraceBalance(code, "typescript")
	}

	dir, err := os.MkdirTemp("", "acb-syntax-ts-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "bot.ts"), []byte(code), 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	// Minimal tsconfig so tsc accepts a single file without a project.
	tsconfig := `{"compilerOptions":{"target":"ES2020","module":"commonjs","strict":false,"noEmit":true},"files":["bot.ts"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o600); err != nil {
		return fmt.Errorf("write tsconfig: %w", err)
	}

	cmd := exec.CommandContext(ctx, "tsc", "--project", filepath.Join(dir, "tsconfig.json"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("typescript syntax: %s", truncate(string(out), 512))
	}
	return nil
}

// checkJavaSyntax compiles with javac (syntax pass only; output discarded).
func checkJavaSyntax(ctx context.Context, code string) error {
	className := extractJavaPublicClass(code)
	if className == "" {
		className = "Bot"
	}
	return checkWithTempFile(ctx, code, className+".java",
		func(path string) *exec.Cmd {
			// -Xlint:none suppresses lint warnings so only errors appear.
			return exec.CommandContext(ctx, "javac", "-Xlint:none", path)
		})
}

// checkWithTempFile writes code to a temp directory as filename, then runs
// the command returned by cmdFn(filePath) and returns its error, if any.
func checkWithTempFile(ctx context.Context, code, filename string, cmdFn func(string) *exec.Cmd) error {
	dir, err := os.MkdirTemp("", "acb-syntax-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	cmd := cmdFn(path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		if msg == "" {
			return fmt.Errorf("syntax check failed: %w", err)
		}
		return fmt.Errorf("%s", truncate(msg, 512))
	}
	return nil
}

// extractJavaPublicClass returns the name of the first public class in src.
var javaPublicClassRe = regexp.MustCompile(`(?m)^\s*public\s+class\s+(\w+)`)

func extractJavaPublicClass(src string) string {
	m := javaPublicClassRe.FindStringSubmatch(src)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// checkBraceBalance is a last-resort fallback that verifies { } are balanced.
func checkBraceBalance(code, lang string) error {
	depth := 0
	for _, ch := range code {
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return fmt.Errorf("%s syntax: unexpected '}'", lang)
			}
		}
	}
	if depth != 0 {
		return fmt.Errorf("%s syntax: unmatched '{' (depth %d at EOF)", lang, depth)
	}
	return nil
}

// truncate limits s to at most n runes, appending "…" when truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

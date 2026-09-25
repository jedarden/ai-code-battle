package conformance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// RepoRoot resolves the repository root from the current working directory,
// walking upward so the harness works both from the repo root (CLI) and from
// the package directory (go test).
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isRepoRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no repository root found above %s (want a directory containing go.mod, bots/ and starters/)", dir)
		}
		dir = parent
	}
}

func isRepoRoot(dir string) bool {
	for _, marker := range []string{"go.mod", "bots", "starters"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return false
		}
	}
	return true
}

// SuiteClient returns an HTTP client that never follows redirects (health
// responses must be a direct 200) and bounds each request.
func SuiteClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Instance is one booted bot process.
type Instance struct {
	cmd    *exec.Cmd
	Port   int
	cancel context.CancelFunc
	done   chan error
}

// BootTarget builds (unless already built) and boots a target, waiting for
// its health endpoint. The caller owns the returned instance and must Stop it.
func BootTarget(ctx context.Context, root string, t Target, secret string) (*Instance, error) {
	if skip := t.DescribeSkips(); skip != "" {
		return nil, fmt.Errorf("target not runnable here: %s", skip)
	}
	dir := filepath.Join(root, t.Dir())

	if len(t.Build) > 0 {
		buildCtx := ctx
		if t.BuildTimeoutHint > 0 {
			var cancel context.CancelFunc
			buildCtx, cancel = context.WithTimeout(ctx, t.BuildTimeoutHint)
			defer cancel()
		}
		build := exec.CommandContext(buildCtx, t.Build[0], t.Build[1:]...)
		build.Dir = dir
		build.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
		if out, err := build.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("build %s: %v\n%s", t.Name, err, tail(out, 4000))
		}
	}

	if t.Binary != "" {
		bin, err := resolveCargoBinary(dir, t.Binary)
		if err != nil {
			return nil, fmt.Errorf("locate %s binary: %w", t.Name, err)
		}
		t.Run = []string{bin}
	}

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, t.Run[0], t.Run[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=%d", t.EnvVarNames.Port, port),
		fmt.Sprintf("%s=%s", t.EnvVarNames.Secret, secret),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", t.Name, err)
	}

	inst := &Instance{cmd: cmd, Port: port, cancel: cancel, done: make(chan error, 1)}
	go func() { inst.done <- cmd.Wait() }()

	if err := inst.awaitHealthy(ctx, 30*time.Second); err != nil {
		inst.Stop()
		return nil, fmt.Errorf("boot %s: %w", t.Name, err)
	}
	return inst, nil
}

func (i *Instance) awaitHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", i.Port)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-i.done:
			return fmt.Errorf("process exited during startup: %v", err)
		default:
		}
		resp, err := SuiteClient(2 * time.Second).Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("health check timed out after %s (last error: %v)", timeout, lastErr)
}

// Stop terminates the bot process group.
func (i *Instance) Stop() {
	if i.cmd.Process != nil {
		// The child runs in its own process group; signal the group so any
		// runtime-spawned helpers die too, then fall back to the direct kill.
		_ = syscall.Kill(-i.cmd.Process.Pid, syscall.SIGKILL)
		_ = i.cmd.Process.Kill()
	}
	i.cancel()
	select {
	case <-i.done:
	case <-time.After(5 * time.Second):
	}
}

// Addr is the base URL for the booted instance.
func (i *Instance) Addr() string { return fmt.Sprintf("http://127.0.0.1:%d", i.Port) }

// RunAgainstTarget boots the target, runs the full suite, and stops it.
func RunAgainstTarget(ctx context.Context, root string, t Target, secret string) (*Instance, SuiteResult, error) {
	inst, err := BootTarget(ctx, root, t, secret)
	if err != nil {
		return nil, SuiteResult{}, err
	}
	defer inst.Stop()
	result := RunSuite(ctx, SuiteClient(10*time.Second), inst.Addr(), secret)
	return inst, result, nil
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("unexpected listener address type")
	}
	return addr.Port, nil
}

// resolveCargoBinary finds the release binary a cargo build produced. The
// effective CARGO_TARGET_DIR varies by environment — plain cargo puts it in
// <dir>/target, while the fleet cargo wrapper redirects build output to a
// shared per-repo directory under /build — so candidates are probed and the
// newest match wins.
func resolveCargoBinary(dir, binary string) (string, error) {
	var candidates []string
	if ctd := os.Getenv("CARGO_TARGET_DIR"); ctd != "" {
		candidates = append(candidates, filepath.Join(ctd, "release", binary))
	}
	candidates = append(candidates, filepath.Join(dir, "target", "release", binary))
	if matches, _ := filepath.Glob("/build/*/release/" + binary); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	newest := ""
	var newestMod time.Time
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if newest == "" || info.ModTime().After(newestMod) {
			newest, newestMod = candidate, info.ModTime()
		}
	}
	if newest == "" {
		return "", fmt.Errorf("not found in any of %s", strings.Join(candidates, ", "))
	}
	return newest, nil
}

func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}

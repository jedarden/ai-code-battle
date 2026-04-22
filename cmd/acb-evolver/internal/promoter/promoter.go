// Package promoter deploys validated+promoted evolved bots to Kubernetes and
// registers them in the ACB bots database.  It also enforces the retirement
// policy: auto-retiring bots below a rating threshold and capping the
// evolved-bot fleet at a configurable population cap.
//
// Promotion flow
//
//  1. Generate a unique bot name (acb-evo-<programID>), bot ID, and secret.
//  2. Write bot source to bots/evolved/<name>/.
//  3. Git add → commit → push (makes source available to Argo Workflow).
//  4. Trigger Argo WorkflowTemplate acb-evolved-bot-deploy which:
//     a. Clones bot source from git
//     b. Builds container image with Kaniko
//     c. Pushes to Forgejo registry
//     d. Creates K8s Secret / Deployment / Service manifests
//     e. Commits manifests to declarative-config repo
//  5. Wait for Argo Workflow to complete (with timeout).
//  6. Insert the bot record directly into the bots database table.
//  7. Record bot_id, bot_name, and bot_secret in the programs table.
//
// Retirement flow
//
//  1. Mark bot as 'retired' in the bots table.
//  2. Delete the K8s manifests and bot source directory from git, commit, push.
//  3. Clear promoted=false / bot_id=NULL in the programs table.
package promoter

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

const (
	botOwner = "acb-evolver"
	botPort  = 8080
)

// Config controls promotion and retirement behaviour.
type Config struct {
	// Registry is the container registry prefix, e.g.
	// "forgejo.ardenone.com/ai-code-battle".
	Registry string

	// RepoDir is the local git repository root used for writing manifests.
	RepoDir string

	// KubectlServer is the kubectl API server URL for deployment polling,
	// e.g. "http://kubectl-ardenone-cluster:8001".
	KubectlServer string

	// Namespace is the Kubernetes namespace where bots are deployed.
	Namespace string

	// EncryptionKey is the hex-encoded AES-256-GCM key used to encrypt
	// secrets before storing them in the bots table.  Empty = plaintext.
	EncryptionKey string

	// DeployWaitTimeout is the maximum time to wait for an ArgoCD-managed
	// deployment to have ≥1 available replica.
	DeployWaitTimeout time.Duration

	// RatingThreshold is the minimum display rating (mu − 2·phi) an evolved
	// bot must maintain to avoid auto-retirement.
	RatingThreshold float64

	// PopCap is the maximum number of simultaneously promoted evolved bots.
	// Lowest-rated bots are retired when the cap is exceeded.
	PopCap int

	// ArgoWorkflowServer is the Argo Workflow API server URL,
	// e.g. "https://argo-ci.ardenone.com".
	ArgoWorkflowServer string

	// ArgoWorkflowNamespace is the namespace where workflows run.
	ArgoWorkflowNamespace string

	// ArgoWorkflowAuthToken is the bearer token for Argo Workflow API auth.
	ArgoWorkflowAuthToken string

	// BotRepo is the git repo URL where bot source is written,
	// e.g. "https://forgejo.ardenone.com/ai-code-battle/ai-code-battle.git".
	BotRepo string

	// BotBranch is the git branch for bot source commits.
	BotBranch string
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		Registry:                "forgejo.ardenone.com/ai-code-battle",
		RepoDir:                 ".",
		KubectlServer:           "http://kubectl-ardenone-cluster:8001",
		Namespace:               "ai-code-battle",
		DeployWaitTimeout:       10 * time.Minute,
		RatingThreshold:         800.0,
		PopCap:                  50,
		ArgoWorkflowServer:      "https://argo-ci.ardenone.com",
		ArgoWorkflowNamespace:   "argo-workflows",
		ArgoWorkflowAuthToken:   "",
		BotRepo:                 "https://forgejo.ardenone.com/ai-code-battle/ai-code-battle.git",
		BotBranch:               "master",
	}
}

// Promoter manages promotion and retirement of evolved bots.
type Promoter struct {
	store *db.Store
	rawDB *sql.DB
	cfg   Config
}

// New creates a Promoter.
func New(store *db.Store, rawDB *sql.DB, cfg Config) *Promoter {
	return &Promoter{store: store, rawDB: rawDB, cfg: cfg}
}

// PromotionResult holds the outcome of a successful promotion.
type PromotionResult struct {
	BotName  string
	BotID    string
	Endpoint string // K8s ClusterIP service URL
}

// Promote deploys a validated candidate as a live evolved bot.
func (p *Promoter) Promote(ctx context.Context, program *db.Program) (*PromotionResult, error) {
	botName := fmt.Sprintf("acb-evo-%d", program.ID)
	_ = fmt.Sprintf("%s/%s:latest", p.cfg.Registry, botName) // image ref built by Argo workflow
	endpoint := fmt.Sprintf("http://%s:%d", botName, botPort)

	botID, err := generateBotID()
	if err != nil {
		return nil, fmt.Errorf("generate bot ID: %w", err)
	}
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	botDir := filepath.Join(p.cfg.RepoDir, "bots", "evolved", botName)
	if err := p.writeBotDir(program, botDir); err != nil {
		return nil, fmt.Errorf("write bot dir: %w", err)
	}

	// Commit bot source to git (required for Argo Workflow to clone it).
	commitMsg := fmt.Sprintf("Add evolved bot source %s (island=%s gen=%d program_id=%d)",
		botName, program.Island, program.Generation, program.ID)
	if err := p.gitCommitPushSource(ctx, botName, commitMsg); err != nil {
		return nil, fmt.Errorf("git commit/push: %w", err)
	}

	// Trigger Argo WorkflowTemplate to build container and create K8s manifests.
	wfName, err := p.triggerArgoWorkflow(ctx, botName, secret, program)
	if err != nil {
		return nil, fmt.Errorf("trigger argo workflow: %w", err)
	}

	// Wait for the workflow to complete (build + manifest commit).
	if err := p.waitForWorkflowCompletion(ctx, wfName); err != nil {
		return nil, fmt.Errorf("workflow completion: %w", err)
	}

	// Wait for the K8s deployment to be ready (ArgoCD sync + pod startup).
	// This polls via kubectl until the deployment reports at least 1 available replica.
	if err := p.waitForDeployment(ctx, botName); err != nil {
		return nil, fmt.Errorf("deployment readiness: %w", err)
	}

	// Insert bot record directly into the bots table (same DB as programs).
	storedSecret := secret
	if p.cfg.EncryptionKey != "" {
		storedSecret, err = encryptAESGCM(secret, p.cfg.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt secret: %w", err)
		}
	}
	_, err = p.rawDB.ExecContext(ctx, `
		INSERT INTO bots (bot_id, name, owner, endpoint_url, shared_secret, status, description, last_active)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, NOW())`,
		botID, botName, botOwner, endpoint, storedSecret,
		fmt.Sprintf("Evolved bot — island=%s gen=%d program_id=%d",
			program.Island, program.Generation, program.ID),
	)
	if err != nil {
		return nil, fmt.Errorf("insert bot record: %w", err)
	}

	if err := p.store.SetPromoted(ctx, program.ID); err != nil {
		return nil, fmt.Errorf("set promoted: %w", err)
	}
	if err := p.store.SetBotID(ctx, program.ID, botID); err != nil {
		return nil, fmt.Errorf("set bot_id: %w", err)
	}
	if err := p.store.SetBotNameAndSecret(ctx, program.ID, botName, secret); err != nil {
		return nil, fmt.Errorf("set bot name/secret: %w", err)
	}

	return &PromotionResult{BotName: botName, BotID: botID, Endpoint: endpoint}, nil
}

// RetireBot marks a bot as retired, removes its K8s manifests, and clears the
// promoted flag in the programs table.
func (p *Promoter) RetireBot(ctx context.Context, programID int64, botID, botName string) error {
	// 1. Mark bot retired in the bots table.
	if _, err := p.rawDB.ExecContext(ctx,
		`UPDATE bots SET status = 'retired' WHERE bot_id = $1`, botID); err != nil {
		return fmt.Errorf("retire bot in DB: %w", err)
	}

	// 2. Remove K8s manifests + bot source from git.
	if botName != "" {
		retireMsg := fmt.Sprintf("Retire evolved bot %s (program_id=%d)", botName, programID)
		if err := p.gitCommitPush(ctx, botName, retireMsg, true); err != nil {
			// Log but don't fail — the bot is already retired in the DB.
			fmt.Printf("promoter: git remove failed for %s: %v\n", botName, err)
		}
	}

	// 3. Clear promoted flag in programs table.
	return p.store.UnsetPromoted(ctx, programID)
}

// RetiredCandidate describes a bot that was auto-retired by EnforcePolicy.
type RetiredCandidate struct {
	ProgramID     int64
	BotID         string
	BotName       string
	DisplayRating float64
	Reason        string
}

// EnforcePolicy auto-retires evolved bots that meet any of these criteria:
//   1. Display rating below cfg.RatingThreshold (bottom 10%)
//   2. 7 consecutive days below rating threshold (per rating_history)
//   3. Population cap exceeded (cfg.PopCap)
// The slice is ordered lowest-rated first so the weakest bots are retired
// first when enforcing the cap.
func (p *Promoter) EnforcePolicy(ctx context.Context) ([]RetiredCandidate, error) {

	// First, get bots with 7 consecutive days below threshold
	consecutiveBotIDs, err := p.queryConsecutiveLowRating(ctx)
	if err != nil {
		return nil, fmt.Errorf("query consecutive low rating: %w", err)
	}
	consecutiveSet := make(map[string]bool)
	for _, botID := range consecutiveBotIDs {
		consecutiveSet[botID] = true
	}
	rows, err := p.rawDB.QueryContext(ctx, `
		SELECT p.id, p.bot_id, COALESCE(p.bot_name, ''),
		       b.rating_mu - 2*b.rating_phi AS display_rating
		FROM programs p
		JOIN bots b ON p.bot_id = b.bot_id
		WHERE p.promoted = TRUE
		  AND p.bot_id IS NOT NULL
		  AND b.status = 'active'
		  AND b.owner = $1
		ORDER BY display_rating ASC`, botOwner)
	if err != nil {
		return nil, fmt.Errorf("query promoted bots: %w", err)
	}
	defer rows.Close()

	type botRow struct {
		programID     int64
		botID         string
		botName       string
		displayRating float64
	}
	var bots []botRow
	for rows.Next() {
		var b botRow
		if err := rows.Scan(&b.programID, &b.botID, &b.botName, &b.displayRating); err != nil {
			return nil, fmt.Errorf("scan bot: %w", err)
		}
		bots = append(bots, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Decide which bots to retire (lowest-rated first).
	remaining := len(bots)
	var toRetire []RetiredCandidate
	for _, b := range bots {
		var reason string
		if consecutiveSet[b.botID] {
			reason = fmt.Sprintf("7 consecutive days below rating %.0f",
				p.cfg.RatingThreshold)
		} else if b.displayRating < p.cfg.RatingThreshold {
			reason = fmt.Sprintf("display rating %.0f < threshold %.0f",
				b.displayRating, p.cfg.RatingThreshold)
		} else if remaining > p.cfg.PopCap {
			reason = fmt.Sprintf("population cap %d exceeded (currently %d)",
				p.cfg.PopCap, remaining)
		}
		if reason != "" {
			toRetire = append(toRetire, RetiredCandidate{
				ProgramID:     b.programID,
				BotID:         b.botID,
				BotName:       b.botName,
				DisplayRating: b.displayRating,
				Reason:        reason,
			})
			remaining--
		}
	}

	for i := range toRetire {
		r := &toRetire[i]
		if err := p.RetireBot(ctx, r.ProgramID, r.BotID, r.BotName); err != nil {
			return toRetire[:i], fmt.Errorf("retire bot %s: %w", r.BotID, err)
		}
	}
	return toRetire, nil
}

// ── file writing ─────────────────────────────────────────────────────────────

func (p *Promoter) writeBotDir(program *db.Program, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	switch program.Language {
	case "go":
		if err := os.WriteFile(filepath.Join(dir, "bot.go"), []byte(program.Code), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module bot\n\ngo 1.24.3\n"), 0o644)
	case "python":
		return os.WriteFile(filepath.Join(dir, "bot.py"), []byte(program.Code), 0o644)
	case "rust":
		if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte(program.Code), 0o644); err != nil {
			return err
		}
		cargoTOML := "[package]\nname = \"bot\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"
		return os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargoTOML), 0o644)
	case "typescript":
		return os.WriteFile(filepath.Join(dir, "bot.ts"), []byte(program.Code), 0o644)
	case "java":
		return os.WriteFile(filepath.Join(dir, "Bot.java"), []byte(program.Code), 0o644)
	case "php":
		return os.WriteFile(filepath.Join(dir, "bot.php"), []byte(program.Code), 0o644)
	default:
		return fmt.Errorf("unsupported language: %s", program.Language)
	}
}

// dockerfileFor returns a single-file Dockerfile for the given language.
func dockerfileFor(language string) (string, error) {
	switch language {
	case "go":
		return `FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.mod
COPY bot.go bot.go
RUN go build -o bot .

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/bot .
ENV BOT_PORT=8080
ENV BOT_SECRET=""
EXPOSE 8080
CMD ["./bot"]
`, nil
	case "python":
		return `FROM python:3.12-slim
WORKDIR /app
COPY bot.py .
ENV BOT_PORT=8080
ENV BOT_SECRET=""
EXPOSE 8080
CMD ["python3", "bot.py"]
`, nil
	case "rust":
		return `FROM rust:1.85-alpine AS builder
WORKDIR /app
COPY Cargo.toml Cargo.toml
COPY src ./src
RUN cargo build --release

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/target/release/bot .
ENV BOT_PORT=8080
ENV BOT_SECRET=""
EXPOSE 8080
CMD ["./bot"]
`, nil
	case "typescript":
		return `FROM node:22-alpine AS builder
WORKDIR /app
COPY bot.ts .
RUN npm install -g typescript && tsc --target ES2020 --module commonjs bot.ts

FROM node:22-alpine
WORKDIR /app
COPY --from=builder /app/bot.js .
ENV BOT_PORT=8080
ENV BOT_SECRET=""
EXPOSE 8080
CMD ["node", "bot.js"]
`, nil
	case "java":
		return `FROM eclipse-temurin:21-alpine AS builder
WORKDIR /app
COPY Bot.java .
RUN javac Bot.java

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=builder /app/*.class .
ENV BOT_PORT=8080
ENV BOT_SECRET=""
EXPOSE 8080
CMD ["java", "Bot"]
`, nil
	case "php":
		return `FROM php:8.3-cli-alpine
WORKDIR /app
COPY bot.php .
ENV BOT_PORT=8080
ENV BOT_SECRET=""
EXPOSE 8080
CMD ["php", "bot.php"]
`, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

// manifestData is the template context for K8s YAML generation.
type manifestData struct {
	Name         string
	Namespace    string
	Island       string
	Generation   int
	Registry     string
	Port         int
	SecretBase64 string
}

var secretManifestTmpl = template.Must(template.New("secret").Parse(`apiVersion: v1
kind: Secret
metadata:
  name: {{.Name}}-secret
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/name: {{.Name}}
    app.kubernetes.io/part-of: ai-code-battle
    app.kubernetes.io/component: evolved-bot
type: Opaque
data:
  bot-secret: {{.SecretBase64}}
`))

var deployManifestTmpl = template.Must(template.New("deploy").Parse(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/name: {{.Name}}
    app.kubernetes.io/part-of: ai-code-battle
    app.kubernetes.io/component: evolved-bot
    acb/island: {{.Island}}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: {{.Name}}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{.Name}}
        app.kubernetes.io/part-of: ai-code-battle
        app.kubernetes.io/component: evolved-bot
        acb/island: {{.Island}}
    spec:
      containers:
        - name: bot
          image: {{.Registry}}/{{.Name}}:latest
          env:
            - name: BOT_PORT
              value: "{{.Port}}"
            - name: BOT_SECRET
              valueFrom:
                secretKeyRef:
                  name: {{.Name}}-secret
                  key: bot-secret
          ports:
            - name: http
              containerPort: {{.Port}}
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 5
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 3
            periodSeconds: 10
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              memory: 128Mi
      restartPolicy: Always
`))

var svcManifestTmpl = template.Must(template.New("svc").Parse(`apiVersion: v1
kind: Service
metadata:
  name: {{.Name}}
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/name: {{.Name}}
    app.kubernetes.io/part-of: ai-code-battle
    app.kubernetes.io/component: evolved-bot
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: {{.Name}}
  ports:
    - name: http
      port: {{.Port}}
      targetPort: http
      protocol: TCP
`))

func (p *Promoter) writeManifests(botName, secret string, program *db.Program) error {
	data := manifestData{
		Name:         botName,
		Namespace:    p.cfg.Namespace,
		Island:       program.Island,
		Generation:   program.Generation,
		Registry:     p.cfg.Registry,
		Port:         botPort,
		SecretBase64: base64.StdEncoding.EncodeToString([]byte(secret)),
	}

	// Write Dockerfile into the bot source directory (already created by writeBotDir).
	dockerfile, err := dockerfileFor(program.Language)
	if err != nil {
		return fmt.Errorf("dockerfile: %w", err)
	}
	botDir := filepath.Join(p.cfg.RepoDir, "bots", "evolved", botName)
	if err := os.WriteFile(filepath.Join(botDir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}

	// K8s Secret
	secretsDir := filepath.Join(p.cfg.RepoDir, "deploy", "k8s", "secrets")
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		return err
	}
	if err := renderToFile(filepath.Join(secretsDir, botName+".yaml"), secretManifestTmpl, data); err != nil {
		return fmt.Errorf("secret manifest: %w", err)
	}

	// K8s Deployment
	deployDir := filepath.Join(p.cfg.RepoDir, "deploy", "k8s", "deployments")
	if err := renderToFile(filepath.Join(deployDir, botName+".yaml"), deployManifestTmpl, data); err != nil {
		return fmt.Errorf("deployment manifest: %w", err)
	}

	// K8s Service
	svcDir := filepath.Join(p.cfg.RepoDir, "deploy", "k8s", "services")
	if err := renderToFile(filepath.Join(svcDir, botName+".yaml"), svcManifestTmpl, data); err != nil {
		return fmt.Errorf("service manifest: %w", err)
	}

	return nil
}

func renderToFile(path string, tmpl *template.Template, data any) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// ── git operations ────────────────────────────────────────────────────────────

// gitCommitPushSource stages, commits, and pushes only the bot source code.
// The Argo Workflow will handle K8s manifests separately.
func (p *Promoter) gitCommitPushSource(ctx context.Context, botName, msg string) error {
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = p.cfg.RepoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
		}
		return nil
	}

	botPath := filepath.Join("bots", "evolved", botName)
	if err := run("add", "--", botPath); err != nil {
		return err
	}

	// Skip commit if nothing changed.
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = p.cfg.RepoDir
	out, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}

	if err := run("commit", "-m", msg); err != nil {
		return err
	}
	return run("push", "origin", p.cfg.BotBranch)
}

// gitCommitPush stages, commits, and pushes changes to git. For retirement,
// it removes the bot source directory. The remove flag indicates whether to
// remove files (true for retirement) or add them (false for promotion).
func (p *Promoter) gitCommitPush(ctx context.Context, botName, msg string, remove bool) error {
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = p.cfg.RepoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
		}
		return nil
	}

	botPath := filepath.Join("bots", "evolved", botName)
	if remove {
		// Remove the bot source directory
		if err := run("rm", "-rf", "--", botPath); err != nil {
			return err
		}
		if err := run("add", "-u", "--", botPath); err != nil {
			return err
		}
	} else {
		if err := run("add", "--", botPath); err != nil {
			return err
		}
	}

	// Skip commit if nothing changed.
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = p.cfg.RepoDir
	out, _ := statusCmd.Output()
	if len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}

	if err := run("commit", "-m", msg); err != nil {
		return err
	}
	return run("push", "origin", p.cfg.BotBranch)
}

// ── Argo Workflow trigger ───────────────────────────────────────────────────────

// triggerArgoWorkflow submits the acb-evolved-bot-deploy WorkflowTemplate
// with parameters for the bot being promoted. Returns the workflow name.
func (p *Promoter) triggerArgoWorkflow(ctx context.Context, botName, secret string, program *db.Program) (string, error) {
	if p.cfg.ArgoWorkflowServer == "" {
		return "", fmt.Errorf("argo workflow server not configured")
	}

	// Build workflow submission parameters.
	wfName := fmt.Sprintf("acb-evo-deploy-%d", time.Now().Unix())
	botPath := fmt.Sprintf("bots/evolved/%s", botName)
	secretB64 := base64.StdEncoding.EncodeToString([]byte(secret))

	wfSpec := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]string{
			"name":      wfName,
			"namespace": p.cfg.ArgoWorkflowNamespace,
		},
		"spec": map[string]any{
			"workflowTemplateRef": map[string]string{
				"name": "acb-evolved-bot-deploy",
			},
			"entrypoint": "deploy-evolved-bot",
			"arguments": map[string]any{
				"parameters": []map[string]string{
					{"name": "bot_name", "value": botName},
					{"name": "bot_secret", "value": secretB64},
					{"name": "language", "value": program.Language},
					{"name": "island", "value": program.Island},
					{"name": "generation", "value": fmt.Sprintf("%d", program.Generation)},
					{"name": "program_id", "value": fmt.Sprintf("%d", program.ID)},
					{"name": "bot_repo", "value": p.cfg.BotRepo},
					{"name": "bot_branch", "value": p.cfg.BotBranch},
					{"name": "bot_path", "value": botPath},
				},
			},
		},
	}

	// Marshal to JSON.
	wfJSON, err := json.Marshal(wfSpec)
	if err != nil {
		return "", fmt.Errorf("marshal workflow: %w", err)
	}

	// Submit workflow via Argo API.
	url := fmt.Sprintf("%s/api/v1/workflows/%s", p.cfg.ArgoWorkflowServer, p.cfg.ArgoWorkflowNamespace)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(wfJSON))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.cfg.ArgoWorkflowAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.ArgoWorkflowAuthToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("submit workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("workflow submission failed (status %d): %s", resp.StatusCode, string(body))
	}

	fmt.Printf("promoter: triggered Argo Workflow %s for bot %s\n", wfName, botName)
	return wfName, nil
}

// ── workflow completion polling ───────────────────────────────────────────────────

// waitForWorkflowCompletion polls the Argo Workflow API until the workflow
// completes (success or failure) or times out.
func (p *Promoter) waitForWorkflowCompletion(ctx context.Context, wfName string) error {
	if p.cfg.ArgoWorkflowServer == "" {
		return fmt.Errorf("argo workflow server not configured")
	}

	deadline := time.Now().Add(30 * time.Minute)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Printf("promoter: waiting for Argo Workflow %s to complete (timeout=30m)…\n", wfName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, phase, err := p.getWorkflowStatus(ctx, wfName)
			if err != nil {
				fmt.Printf("promoter: workflow poll error: %v\n", err)
				if time.Now().After(deadline) {
					return fmt.Errorf("workflow poll timeout after error: %w", err)
				}
				continue
			}

			fmt.Printf("promoter: workflow %s status=%s phase=%s\n", wfName, status, phase)

			switch phase {
			case "Succeeded":
				fmt.Printf("promoter: workflow %s completed successfully\n", wfName)
				return nil
			case "Failed", "Error":
				return fmt.Errorf("workflow %s failed with phase %s (status: %s)", wfName, phase, status)
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("workflow %s did not complete after 30 minutes (last phase: %s)", wfName, phase)
			}
		}
	}
}

// getWorkflowStatus fetches the current status and phase of a workflow.
func (p *Promoter) getWorkflowStatus(ctx context.Context, wfName string) (status, phase string, err error) {
	url := fmt.Sprintf("%s/api/v1/workflows/%s/%s", p.cfg.ArgoWorkflowServer, p.cfg.ArgoWorkflowNamespace, wfName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	if p.cfg.ArgoWorkflowAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.cfg.ArgoWorkflowAuthToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("get workflow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var wfResp struct {
		Status struct {
			Phase   string `json:"phase"`
			StartedAt string `json:"startedAt"`
			FinishedAt string `json:"finishedAt"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wfResp); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}

	status = "running"
	if wfResp.Status.FinishedAt != "" {
		status = "finished"
	}

	return status, wfResp.Status.Phase, nil
}

// ── deployment readiness ──────────────────────────────────────────────────────

func (p *Promoter) waitForDeployment(ctx context.Context, name string) error {
	deadline := time.Now().Add(p.cfg.DeployWaitTimeout)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	fmt.Printf("promoter: waiting for deployment %s to be ready (timeout=%s)…\n",
		name, p.cfg.DeployWaitTimeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			n, err := p.availableReplicas(ctx, name)
			if err != nil {
				fmt.Printf("promoter: kubectl poll error: %v\n", err)
			} else if n >= 1 {
				fmt.Printf("promoter: deployment %s ready (%d replica)\n", name, n)
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("deployment %s not ready after %s", name, p.cfg.DeployWaitTimeout)
			}
		}
	}
}

func (p *Promoter) availableReplicas(ctx context.Context, name string) (int, error) {
	cmd := exec.CommandContext(ctx, "kubectl",
		"--server="+p.cfg.KubectlServer,
		"get", "deployment", name,
		"-n", p.cfg.Namespace,
		"-o", "jsonpath={.status.availableReplicas}",
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, nil
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n, nil
}

// ── container image build ─────────────────────────────────────────────────────

func (p *Promoter) buildAndPushImage(ctx context.Context, botDir, image string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not in PATH")
	}
	build := exec.CommandContext(ctx, "docker", "build", "-t", image, botDir)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %s", truncate(string(out), 512))
	}
	push := exec.CommandContext(ctx, "docker", "push", image)
	if out, err := push.CombinedOutput(); err != nil {
		return fmt.Errorf("docker push: %s", truncate(string(out), 512))
	}
	return nil
}

// ── crypto helpers ────────────────────────────────────────────────────────────

func generateBotID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "b_" + hex.EncodeToString(b), nil
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func encryptAESGCM(plaintext, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 32 {
		return "", fmt.Errorf("invalid AES-256-GCM key (must be 64 hex chars)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ct), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func init() {
	// register queryConsecutiveLowRating for retirement automation
}

// queryConsecutiveLowRating returns bot_ids that have been below the rating
// threshold for 7 consecutive days. Uses rating_history to track daily ratings.
func (p *Promoter) queryConsecutiveLowRating(ctx context.Context) ([]string, error) {
	// Get the latest rating for each day (using DISTINCT ON for per-day records)
	// then check for 7 consecutive days all below threshold.
	query := `
		WITH daily_ratings AS (
			SELECT DISTINCT
				bot_id,
				DATE(recorded_at) AS rating_date,
				rating
			FROM rating_history
			WHERE DATE(recorded_at) >= CURRENT_DATE - INTERVAL '14 days'
		),
		consecutive_counts AS (
			SELECT
				bot_id,
				rating_date,
				rating,
				// Count consecutive days below threshold ending at this date
				SUM(CASE WHEN rating < $1 THEN 1 ELSE 0 END) OVER (
					PARTITION BY bot_id
					ORDER BY rating_date DESC
					ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
				) AS consecutive_below
			FROM daily_ratings
		)
		SELECT DISTINCT bot_id
		FROM consecutive_counts
		WHERE consecutive_below >= 7
	`

	rows, err := p.rawDB.QueryContext(ctx, query, p.cfg.RatingThreshold)
	if err != nil {
		return nil, fmt.Errorf("query consecutive low rating: %w", err)
	}
	defer rows.Close()

	var botIDs []string
	for rows.Next() {
		var botID string
		if err := rows.Scan(&botID); err != nil {
			return nil, fmt.Errorf("scan bot_id: %w", err)
		}
		botIDs = append(botIDs, botID)
	}
	return botIDs, rows.Err()
}

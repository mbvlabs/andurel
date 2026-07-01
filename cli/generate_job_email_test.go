package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestGenerateJobWritesJobWorkerAndRegistration(t *testing.T) {
	rootDir := setupGenerateFileTestProject(t)

	if err := generateJob("ProcessPayment", "financial"); err != nil {
		t.Fatalf("generateJob failed: %v", err)
	}

	jobContent := readGeneratedTestFile(t, rootDir, "queue/jobs/process_payment.go")
	for _, want := range []string{
		"type ProcessPaymentArgs struct{}",
		"func (ProcessPaymentArgs) Kind() string { return \"process_payment\" }",
		"func (ProcessPaymentArgs) InsertOpts() river.InsertOpts",
		"Queue: \"financial\"",
	} {
		if !strings.Contains(jobContent, want) {
			t.Fatalf("job file should contain %q\n\n%s", want, jobContent)
		}
	}

	workerContent := readGeneratedTestFile(t, rootDir, "queue/workers/process_payment.go")
	for _, want := range []string{
		"\"example.com/app/queue/jobs\"",
		"type ProcessPaymentWorker struct",
		"func NewProcessPaymentWorker() *ProcessPaymentWorker",
		"river.WorkerDefaults[jobs.ProcessPaymentArgs]",
	} {
		if !strings.Contains(workerContent, want) {
			t.Fatalf("worker file should contain %q\n\n%s", want, workerContent)
		}
	}

	workersContent := readGeneratedTestFile(t, rootDir, "queue/workers/workers.go")
	for _, want := range []string{
		"river.AddWorkerSafely(wrks, NewProcessPaymentWorker())",
		"// andurel:worker-registration-point",
	} {
		if !strings.Contains(workersContent, want) {
			t.Fatalf("workers registration should contain %q\n\n%s", want, workersContent)
		}
	}
}

func TestGenerateJobDefaultQueueOmitsInsertOpts(t *testing.T) {
	rootDir := setupGenerateFileTestProject(t)

	if err := generateJob("SendWelcomeEmail", ""); err != nil {
		t.Fatalf("generateJob failed: %v", err)
	}

	jobContent := readGeneratedTestFile(t, rootDir, "queue/jobs/send_welcome_email.go")
	if strings.Contains(jobContent, "InsertOpts") {
		t.Fatalf("default queue job should not include InsertOpts\n\n%s", jobContent)
	}
	if !strings.Contains(jobContent, "func (SendWelcomeEmailArgs) Kind() string { return \"send_welcome_email\" }") {
		t.Fatalf("job file should include kind\n\n%s", jobContent)
	}
}

func TestGenerateJobUberFXWritesQueueWorkerAndModuleRegistration(t *testing.T) {
	rootDir := setupGenerateFileTestProject(t)
	writeGenerateFileTestLock(t, rootDir, "uberfx")
	workersPath := filepath.Join(rootDir, "queue", "workers.go")
	if err := os.MkdirAll(filepath.Dir(workersPath), 0o755); err != nil {
		t.Fatalf("create queue dir: %v", err)
	}
	if err := os.WriteFile(workersPath, []byte(fxWorkersFixture), 0o644); err != nil {
		t.Fatalf("write fx workers fixture: %v", err)
	}

	if err := generateJob("ProcessPayment", "financial"); err != nil {
		t.Fatalf("generateJob failed: %v", err)
	}

	workerContent := readGeneratedTestFile(t, rootDir, "queue/process_payment.go")
	for _, want := range []string{
		"package queue",
		"\"example.com/app/queue/jobs\"",
		"type ProcessPaymentWorker struct",
		"func NewProcessPaymentWorker() *ProcessPaymentWorker",
		"func (w *ProcessPaymentWorker) Register(workers *river.Workers) error",
		"return river.AddWorkerSafely(workers, w)",
		"river.WorkerDefaults[jobs.ProcessPaymentArgs]",
	} {
		if !strings.Contains(workerContent, want) {
			t.Fatalf("worker file should contain %q\n\n%s", want, workerContent)
		}
	}

	if _, err := os.Stat(filepath.Join(rootDir, "queue", "workers", "process_payment.go")); !os.IsNotExist(err) {
		t.Fatalf("uberfx job generation should not write queue/workers/process_payment.go: %v", err)
	}

	workersContent := readGeneratedTestFile(t, rootDir, "queue/workers.go")
	for _, want := range []string{
		"NewProcessPaymentWorker,",
		"fx.Invoke(func(workers *river.Workers, worker *ProcessPaymentWorker) error",
		"return worker.Register(workers)",
	} {
		if !strings.Contains(workersContent, want) {
			t.Fatalf("fx workers registration should contain %q\n\n%s", want, workersContent)
		}
	}
	for _, marker := range []string{
		"// andurel:worker-constructor-registration-point",
		"// andurel:worker-fx-invoke-registration-point",
	} {
		if strings.Contains(workersContent, marker) {
			t.Fatalf("fx workers registration should not contain marker %q\n\n%s", marker, workersContent)
		}
	}
}

func TestGenerateEmailWritesTemplTransformer(t *testing.T) {
	rootDir := setupGenerateFileTestProject(t)

	if err := generateEmail("WelcomeEmail"); err != nil {
		t.Fatalf("generateEmail failed: %v", err)
	}

	content := readGeneratedTestFile(t, rootDir, "email/welcome_email.templ")
	for _, want := range []string{
		"package email",
		"type WelcomeEmail struct",
		"var _ Transformer = (*WelcomeEmail)(nil)",
		"func (e WelcomeEmail) ToHTML() (string, error)",
		"templ (e WelcomeEmail) render()",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("email file should contain %q\n\n%s", want, content)
		}
	}
}

func setupGenerateFileTestProject(t *testing.T) string {
	t.Helper()

	cache.ClearFileSystemCache()
	t.Cleanup(cache.ClearFileSystemCache)

	rootDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte("module example.com/app\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	workersPath := filepath.Join(rootDir, "queue", "workers", "workers.go")
	if err := os.MkdirAll(filepath.Dir(workersPath), 0o755); err != nil {
		t.Fatalf("create workers dir: %v", err)
	}
	if err := os.WriteFile(workersPath, []byte(workersFixture), 0o644); err != nil {
		t.Fatalf("write workers fixture: %v", err)
	}

	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir temp project: %v", err)
	}

	return rootDir
}

func writeGenerateFileTestLock(t *testing.T, rootDir, diMode string) {
	t.Helper()

	content := `{
  "version": "test",
  "tools": {},
  "scaffoldConfig": {
    "projectName": "app",
    "database": "postgresql",
    "cssFramework": "tailwind",
    "diMode": "` + diMode + `"
  }
}
`
	if err := os.WriteFile(filepath.Join(rootDir, "andurel.lock"), []byte(content), 0o644); err != nil {
		t.Fatalf("write andurel.lock: %v", err)
	}
}

func readGeneratedTestFile(t *testing.T, rootDir, relPath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(rootDir, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	return string(content)
}

const workersFixture = `package workers

import "github.com/riverqueue/river"

func Register() (*river.Workers, error) {
	wrks := river.NewWorkers()
	// andurel:worker-registration-point
	return wrks, nil
}
`

const fxWorkersFixture = `package queue

import (
	"github.com/riverqueue/river"
	"go.uber.org/fx"
)

var wrksConstructors = fx.Provide(
	river.NewWorkers,
	NewSendTransactionalEmailWorker,
	NewSendMarketingEmailWorker,
)

var WorkersModule = fx.Module(
	"queue-workers",
	wrksConstructors,
	fx.Invoke(func(workers *river.Workers, worker *SendTransactionalEmailWorker) error {
		return worker.Register(workers)
	}),
	fx.Invoke(func(workers *river.Workers, worker *SendMarketingEmailWorker) error {
		return worker.Register(workers)
	}),
)
`

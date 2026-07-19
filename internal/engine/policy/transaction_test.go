package policy

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPolicyTransactionAtomicReplaceChecksHashAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "safeslop.cue")
	old := []byte("old")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	validated := false
	outcome, err := ReplacePolicyAtomic(path, []byte("new"), TransactionOptions{ExpectedHash: transactionSHA256Hex(old), Validate: func(b []byte) error { validated = string(b) == "new"; return nil }})
	if err != nil || outcome != TransactionKnownCommitted || !validated {
		t.Fatalf("replace = %s %v validated=%v", outcome, err, validated)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("file = %q", got)
	}
	outcome, err = ReplacePolicyAtomic(path, []byte("bad"), TransactionOptions{ExpectedHash: transactionSHA256Hex(old)})
	if !errors.Is(err, ErrPolicyStale) || outcome != TransactionKnownFailed {
		t.Fatalf("stale = %s %v", outcome, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("stale changed file = %q", got)
	}
}

func TestPolicyTransactionCreateOnlyAndValidationFailureDoNotWrite(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.cue")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome, err := ReplacePolicyAtomic(existing, []byte("new"), TransactionOptions{CreateOnly: true})
	if !errors.Is(err, ErrPolicyExists) || outcome != TransactionKnownFailed {
		t.Fatalf("create existing = %s %v", outcome, err)
	}
	if got, _ := os.ReadFile(existing); string(got) != "old" {
		t.Fatalf("create-only changed file = %q", got)
	}

	created := filepath.Join(dir, "created.cue")
	outcome, err = ReplacePolicyAtomic(created, []byte("new"), TransactionOptions{CreateOnly: true, Validate: func([]byte) error { return errors.New("invalid") }})
	if err == nil || outcome != TransactionKnownFailed {
		t.Fatalf("invalid create = %s %v", outcome, err)
	}
	if _, err := os.Stat(created); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid create wrote file: %v", err)
	}

	outcome, err = ReplacePolicyAtomic(created, []byte("new"), TransactionOptions{CreateOnly: true})
	if err != nil || outcome != TransactionKnownCommitted {
		t.Fatalf("create = %s %v", outcome, err)
	}
	if got, _ := os.ReadFile(created); string(got) != "new" {
		t.Fatalf("created = %q", got)
	}
}

func TestPolicyTransactionCommitUncertainIsNotSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "safeslop.cue")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	outcome, err := replacePolicyAtomic(path, []byte("new"), TransactionOptions{}, &transactionHooks{syncDir: func(string) error { return errors.New("dir sync") }})
	if !errors.Is(err, ErrPolicyCommitUncertain) || outcome != TransactionCommitUncertain {
		t.Fatalf("uncertain = %s %v", outcome, err)
	}
}

func TestPolicyTransactionSerializesContendingWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "safeslop.cue")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	inCritical := false
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := replacePolicyAtomic(path, []byte("new"), TransactionOptions{}, &transactionHooks{syncFile: func(f *os.File) error {
				mu.Lock()
				if inCritical {
					t.Errorf("policy transaction overlapped")
				}
				inCritical = true
				inCritical = false
				mu.Unlock()
				return f.Sync()
			}})
			if err != nil {
				t.Errorf("replace: %v", err)
			}
		}()
	}
	wg.Wait()
}

package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

var (
	ErrPolicyStale           = errors.New("policy file hash does not match expected hash")
	ErrPolicyExists          = errors.New("policy file already exists")
	ErrPolicyCommitUncertain = errors.New("policy file commit is uncertain")
)

type TransactionOutcome string

const (
	TransactionKnownCommitted  TransactionOutcome = "known-committed"
	TransactionKnownFailed     TransactionOutcome = "known-failed"
	TransactionCommitUncertain TransactionOutcome = "commit-uncertain"
)

type TransactionOptions struct {
	ExpectedHash string
	CreateOnly   bool
	Validate     func([]byte) error
}

type transactionHooks struct {
	syncFile func(*os.File) error
	rename   func(string, string) error
	link     func(string, string) error
	syncDir  func(string) error
}

func ReplacePolicyAtomic(path string, content []byte, opts TransactionOptions) (TransactionOutcome, error) {
	return replacePolicyAtomic(path, content, opts, nil)
}

func replacePolicyAtomic(path string, content []byte, opts TransactionOptions, hooks *transactionHooks) (TransactionOutcome, error) {
	if err := withPolicyLock(path, func() error {
		return replacePolicyUnderLock(path, content, opts, hooks)
	}); err != nil {
		if errors.Is(err, ErrPolicyCommitUncertain) {
			return TransactionCommitUncertain, err
		}
		return TransactionKnownFailed, err
	}
	return TransactionKnownCommitted, nil
}

func replacePolicyUnderLock(path string, content []byte, opts TransactionOptions, hooks *transactionHooks) error {
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	exists := err == nil
	if opts.CreateOnly && exists {
		return ErrPolicyExists
	}
	if opts.ExpectedHash != "" && (!exists || transactionSHA256Hex(current) != opts.ExpectedHash) {
		return ErrPolicyStale
	}
	if opts.Validate != nil {
		if err := opts.Validate(content); err != nil {
			return err
		}
	}
	return writePolicyAtomic(path, content, opts.CreateOnly, hooks)
}

func withPolicyLock(path string, fn func() error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "."+filepath.Base(path)+".lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lock.Chmod(0o600); err != nil {
		return err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func writePolicyAtomic(path string, content []byte, createOnly bool, hooks *transactionHooks) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		_ = tmp.Close()
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if _, err := io.Copy(tmp, bytes.NewReader(content)); err != nil {
		return err
	}
	if err := policyHookSyncFile(hooks, tmp); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if createOnly {
		if err := policyHookLink(hooks, tmpPath, path); err != nil {
			return err
		}
		if err := os.Remove(tmpPath); err != nil {
			return ErrPolicyCommitUncertain
		}
		removeTemp = false
	} else {
		if err := policyHookRename(hooks, tmpPath, path); err != nil {
			return err
		}
		removeTemp = false
	}
	if err := policyHookSyncDir(hooks, dir); err != nil {
		return ErrPolicyCommitUncertain
	}
	return nil
}

func transactionSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func policyHookSyncFile(h *transactionHooks, f *os.File) error {
	if h != nil && h.syncFile != nil {
		return h.syncFile(f)
	}
	return f.Sync()
}

func policyHookRename(h *transactionHooks, oldPath, newPath string) error {
	if h != nil && h.rename != nil {
		return h.rename(oldPath, newPath)
	}
	return os.Rename(oldPath, newPath)
}

func policyHookLink(h *transactionHooks, oldPath, newPath string) error {
	if h != nil && h.link != nil {
		return h.link(oldPath, newPath)
	}
	return os.Link(oldPath, newPath)
}

func policyHookSyncDir(h *transactionHooks, dir string) error {
	if h != nil && h.syncDir != nil {
		return h.syncDir(dir)
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

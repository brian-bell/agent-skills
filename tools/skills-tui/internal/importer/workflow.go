package importer

import (
	"context"
	"errors"
	"fmt"
)

// ScanSession retains one validated checkout between candidate selection and
// import. Close releases its temporary repository snapshot.
type ScanSession struct {
	Root          string
	RepositoryURL string
	Commit        string
	Candidates    []Candidate

	checkout *Checkout
}

// Close releases the checkout owned by this scan session.
func (s *ScanSession) Close() error {
	if s == nil || s.checkout == nil {
		return nil
	}
	return s.checkout.Close()
}

// Workflow composes checkout, scan, collision validation, saved history, and
// transactional import behind the TUI-facing service boundary.
type Workflow struct {
	History    HistoryStore
	Checkouts  CheckoutProvider
	Repository RepositoryImporter
}

// SavedRepositories returns picker history in most-recently-used order.
func (w Workflow) SavedRepositories() ([]RepositoryRecord, error) {
	return w.History.List()
}

// DeleteSavedRepository deletes picker history only.
func (w Workflow) DeleteSavedRepository(repositoryURL string) error {
	return w.History.Delete(repositoryURL)
}

// Scan checks out and validates one repository. History is updated only after
// the scan yields at least one valid, non-conflicting candidate.
func (w Workflow) Scan(ctx context.Context, repositoryURL string) (_ *ScanSession, err error) {
	if w.Checkouts == nil {
		return nil, fmt.Errorf("GitHub checkout provider is not configured")
	}
	checkout, err := w.Checkouts.Checkout(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			if cleanupErr := checkout.Close(); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("temporary checkout cleanup failed: %w", cleanupErr))
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	candidates, err := ScanContext(ctx, checkout.Root)
	if err != nil {
		return nil, err
	}
	candidates, err = w.Repository.ValidateCandidates(candidates)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	valid := 0
	for _, candidate := range candidates {
		if candidate.Valid {
			valid++
		}
	}
	if valid == 0 {
		return nil, fmt.Errorf("repository contains no valid, non-conflicting skills")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := w.History.record(ctx, checkout.RepositoryURL); err != nil {
		return nil, fmt.Errorf("save import repository history: %w", err)
	}

	closeOnError = false
	return &ScanSession{
		Root: checkout.Root, RepositoryURL: checkout.RepositoryURL, Commit: checkout.Commit,
		Candidates: candidates, checkout: checkout,
	}, nil
}

// Import transactionally publishes the selected candidates from session. The
// session remains open so callers can retry a failed batch or close it after
// success/cancellation.
func (w Workflow) Import(ctx context.Context, session *ScanSession, selectedIDs []string) ([]string, error) {
	if session == nil {
		return nil, fmt.Errorf("scan session is required")
	}
	return w.Repository.Import(ctx, ImportRequest{
		CheckoutRoot: session.Root, RepositoryURL: session.RepositoryURL, Commit: session.Commit,
		Candidates: session.Candidates, SelectedIDs: selectedIDs,
	})
}

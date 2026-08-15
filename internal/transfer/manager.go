package transfer

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TransferStatus string

const (
	StatusPending     TransferStatus = "pending"
	StatusUploading   TransferStatus = "uploading"
	StatusCompleted   TransferStatus = "completed"
	StatusFailed      TransferStatus = "failed"
	StatusCancelled   TransferStatus = "cancelled"
	StatusDownloading TransferStatus = "downloading"
)

type Transfer struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Size      int64         `json:"size"`
	Path      string        `json:"path,omitempty"`
	Status    TransferStatus `json:"status"`
	Progress  int64         `json:"progress"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`

	mu sync.RWMutex
}

type Manager struct {
	transfers map[string]*Transfer
	mu        sync.RWMutex
	baseDir   string
	log       *slog.Logger
}

func NewManager(tempDir string, log *slog.Logger) (*Manager, error) {
	base := filepath.Join(tempDir, "godrop")

	if err := os.MkdirAll(base, 0700); err != nil {
		return nil, fmt.Errorf("failed to create godrop temp dir: %w", err)
	}

	return &Manager{
		transfers: make(map[string]*Transfer),
		baseDir:   base,
		log:       log,
	}, nil
}

func generateID() (string, error) {
	b := make([]byte, 8)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func (m *Manager) CreateTransfer(name string, size int64) (*Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, err := generateID()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(m.baseDir, id)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create transfer dir: %w", err)
	}

	safeName := filepath.Base(name)

	if safeName == "." || safeName == string(filepath.Separator) || safeName == "" {
		return nil, errors.New("invalid filename")
	}

	filePath := filepath.Join(dir, safeName)

	now := time.Now()

	t := &Transfer{
		ID:        id,
		Name:      safeName,
		Size:      size,
		Path:      filePath,
		Status:    StatusPending,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.transfers[id] = t

	return t, nil
}

func (m *Manager) GetTransfer(id string) (*Transfer, bool) {
	m.mu.RLock()
	t, ok := m.transfers[id]
	m.mu.RUnlock()

	if !ok {
		return nil, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	copy := &Transfer{
		ID:        t.ID,
		Name:      t.Name,
		Size:      t.Size,
		Path:      t.Path,
		Status:    t.Status,
		Progress:  t.Progress,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	return copy, true
}

func (m *Manager) ListTransfers() []*Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Transfer, 0, len(m.transfers))

	for _, t := range m.transfers {
		t.mu.RLock()

		copy := &Transfer{
			ID:        t.ID,
			Name:      t.Name,
			Size:      t.Size,
			Path:      t.Path,
			Status:    t.Status,
			Progress:  t.Progress,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		}

		t.mu.RUnlock()

		list = append(list, copy)
	}

	return list
}

func (m *Manager) DeleteTransfer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[id]
	if !ok {
		return errors.New("transfer not found")
	}

	if err := os.RemoveAll(filepath.Dir(t.Path)); err != nil {
		return err
	}

	delete(m.transfers, id)

	return nil
}

func (m *Manager) UpdateProgress(id string, bytes int64) error {
	m.mu.RLock()
	t, ok := m.transfers[id]
	m.mu.RUnlock()

	if !ok {
		return errors.New("transfer not found")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.Progress = bytes
	t.UpdatedAt = time.Now()

	return nil
}

func (m *Manager) SetStatus(id string, status TransferStatus) error {
	m.mu.RLock()
	t, ok := m.transfers[id]
	m.mu.RUnlock()

	if !ok {
		return errors.New("transfer not found")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = status
	t.UpdatedAt = time.Now()

	return nil
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, t := range m.transfers {
		dir := filepath.Dir(t.Path)

		if err := os.RemoveAll(dir); err != nil {
			m.log.Warn(
				"cleanup: failed to remove transfer dir",
				"id", id,
				"error", err,
			)
		} else {
			m.log.Debug(
				"cleanup: removed transfer",
				"id", id,
			)
		}

		delete(m.transfers, id)
	}

	_ = os.Remove(m.baseDir)
}

func (m *Manager) SaveUploadedFile(id string, reader io.Reader) (int64, error) {
	m.mu.RLock()
	t, ok := m.transfers[id]
	m.mu.RUnlock()

	if !ok {
		return 0, errors.New("transfer not found")
	}

	_ = m.SetStatus(id, StatusUploading)

	f, err := os.Create(t.Path)
	if err != nil {
		_ = m.SetStatus(id, StatusFailed)
		return 0, err
	}

	defer f.Close()

	var written int64

	buf := make([]byte, 32*1024)

	for {
		n, readErr := reader.Read(buf)

		if n > 0 {
			nw, writeErr := f.Write(buf[:n])

			if writeErr != nil {
				_ = m.SetStatus(id, StatusFailed)
				return written, writeErr
			}

			written += int64(nw)
			_ = m.UpdateProgress(id, written)
		}

		if readErr == io.EOF {
			break
		}

		if readErr != nil {
			_ = m.SetStatus(id, StatusFailed)
			return written, readErr
		}
	}

	if t.Size > 0 && written != t.Size {
		_ = m.SetStatus(id, StatusFailed)

		return written, fmt.Errorf(
			"uploaded size %d does not match expected %d",
			written,
			t.Size,
		)
	}

	_ = m.SetStatus(id, StatusCompleted)

	return written, nil
}
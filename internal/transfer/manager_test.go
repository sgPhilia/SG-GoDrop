package transfer

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestManager_CreateTransfer(t *testing.T) {
	mgr, err := NewManager(t.TempDir(), logger.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Cleanup()

	t.Run("creates transfer with unique ID", func(t *testing.T) {
		t1, err := mgr.CreateTransfer("file1.txt", 100)
		if err != nil {
			t.Fatal(err)
		}
		if t1.ID == "" {
			t.Error("expected non-empty ID")
		}
		if t1.Name != "file1.txt" {
			t.Errorf("expected name 'file1.txt', got %q", t1.Name)
		}
		if t1.Status != StatusPending {
			t.Errorf("expected status pending, got %v", t1.Status)
		}
		if _, err := os.Stat(filepath.Dir(t1.Path)); err != nil {
			t.Errorf("transfer directory missing: %v", err)
		}
	})

	t.Run("creates different IDs", func(t *testing.T) {
		t1, _ := mgr.CreateTransfer("a", 1)
		t2, _ := mgr.CreateTransfer("b", 1)
		if t1.ID == t2.ID {
			t.Error("IDs should be unique")
		}
	})

	t.Run("sanitizes filename", func(t *testing.T) {
		t1, _ := mgr.CreateTransfer("../../etc/passwd", 1)
		if t1.Name != "passwd" {
			t.Errorf("expected sanitized name 'passwd', got %q", t1.Name)
		}
	})
}

func TestManager_GetTransfer(t *testing.T) {
	mgr, _ := NewManager(t.TempDir(), logger.Default())
	defer mgr.Cleanup()
	t1, _ := mgr.CreateTransfer("test", 10)

	got, ok := mgr.GetTransfer(t1.ID)
	if !ok {
		t.Fatal("transfer not found")
	}
	if got.ID != t1.ID {
		t.Errorf("expected ID %s, got %s", t1.ID, got.ID)
	}
	if _, ok := mgr.GetTransfer("nonexistent"); ok {
		t.Error("should not find nonexistent transfer")
	}
}

func TestManager_ListTransfers(t *testing.T) {
	mgr, _ := NewManager(t.TempDir(), logger.Default())
	defer mgr.Cleanup()

	list := mgr.ListTransfers()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}

	mgr.CreateTransfer("a", 1)
	mgr.CreateTransfer("b", 2)
	list = mgr.ListTransfers()
	if len(list) != 2 {
		t.Errorf("expected 2 transfers, got %d", len(list))
	}
}

func TestManager_DeleteTransfer(t *testing.T) {
	mgr, _ := NewManager(t.TempDir(), logger.Default())
	defer mgr.Cleanup()

	t1, _ := mgr.CreateTransfer("del", 100)
	dir := filepath.Dir(t1.Path)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("directory not created")
	}

	err := mgr.DeleteTransfer(t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("directory still exists after delete")
	}
	if _, ok := mgr.GetTransfer(t1.ID); ok {
		t.Error("transfer still present after delete")
	}
	if err := mgr.DeleteTransfer(t1.ID); err == nil {
		t.Error("expected error on double delete")
	}
}

func TestManager_UpdateProgressAndStatus(t *testing.T) {
	mgr, _ := NewManager(t.TempDir(), logger.Default())
	defer mgr.Cleanup()

	t1, _ := mgr.CreateTransfer("prog", 1000)

	if err := mgr.UpdateProgress(t1.ID, 500); err != nil {
		t.Fatal(err)
	}
	got, _ := mgr.GetTransfer(t1.ID)
	if got.Progress != 500 {
		t.Errorf("expected progress 500, got %d", got.Progress)
	}

	if err := mgr.SetStatus(t1.ID, StatusCompleted); err != nil {
		t.Fatal(err)
	}
	got, _ = mgr.GetTransfer(t1.ID)
	if got.Status != StatusCompleted {
		t.Errorf("expected status completed, got %v", got.Status)
	}

	if err := mgr.UpdateProgress("bad", 0); err == nil {
		t.Error("expected error on bad ID")
	}
}

func TestManager_SaveUploadedFile(t *testing.T) {
	mgr, _ := NewManager(t.TempDir(), logger.Default())
	defer mgr.Cleanup()

	t1, _ := mgr.CreateTransfer("upload.txt", 1024)
	data := bytes.Repeat([]byte("x"), 1024)
	reader := bytes.NewReader(data)

	n, err := mgr.SaveUploadedFile(t1.ID, reader)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1024 {
		t.Errorf("expected 1024 bytes written, got %d", n)
	}

	got, _ := mgr.GetTransfer(t1.ID)
	if got.Status != StatusCompleted {
		t.Errorf("expected status completed, got %v", got.Status)
	}
	if got.Progress != 1024 {
		t.Errorf("expected progress 1024, got %d", got.Progress)
	}

	content, err := os.ReadFile(t1.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, data) {
		t.Error("file content mismatch")
	}
}

func TestManager_Cleanup(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir, logger.Default())
	t1, _ := mgr.CreateTransfer("clean", 1)
	dirPath := filepath.Dir(t1.Path)
	if _, err := os.Stat(dirPath); err != nil {
		t.Fatal("transfer directory not created")
	}

	mgr.Cleanup()
	if _, err := os.Stat(filepath.Join(dir, "godrop")); !os.IsNotExist(err) {
		t.Error("base godrop directory still exists after cleanup")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr, _ := NewManager(t.TempDir(), logger.Default())
	defer mgr.Cleanup()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mgr.CreateTransfer("file", int64(i+1))
		}(i)
	}
	wg.Wait()

	list := mgr.ListTransfers()
	if len(list) != 10 {
		t.Errorf("expected 10 transfers, got %d", len(list))
	}

	for _, t := range list {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			mgr.UpdateProgress(id, 123)
		}(t.ID)
	}
	wg.Wait()

	for _, t := range list {
		got, _ := mgr.GetTransfer(t.ID)
		if got.Progress != 123 {
			t.Errorf("expected progress 123, got %d", got.Progress)
		}
	}
}
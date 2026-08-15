package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"godrop/internal/config"
	"godrop/internal/logger"
	"godrop/internal/transfer"
)

func TestServer_Index(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.indexHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("GoDrop")) {
		t.Error("response missing 'GoDrop'")
	}
}

func TestServer_UploadDownloadDelete(t *testing.T) {
	srv := newTestServer(t)

	fileContent := []byte("hello world")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write(fileContent)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.uploadHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload failed: %d", resp.StatusCode)
	}
	var uploadResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		t.Fatal(err)
	}
	id, ok := uploadResp["id"].(string)
	if !ok || id == "" {
		t.Fatal("missing id in response")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/transfers", nil)
	w = httptest.NewRecorder()
	srv.transfersHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list failed: %d", w.Code)
	}
	var listResp struct {
		Transfers []transfer.Transfer `json:"transfers"`
	}
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Transfers) != 1 {
		t.Fatalf("expected 1 transfer, got %d", len(listResp.Transfers))
	}
	if listResp.Transfers[0].ID != id {
		t.Errorf("expected ID %s, got %s", id, listResp.Transfers[0].ID)
	}
	if listResp.Transfers[0].Status != transfer.StatusCompleted {
		t.Errorf("expected status completed, got %v", listResp.Transfers[0].Status)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/transfers/"+id+"/download", nil)
	w = httptest.NewRecorder()
	srv.transferHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("download failed: %d", w.Code)
	}
	downloaded, _ := io.ReadAll(w.Body)
	if !bytes.Equal(downloaded, fileContent) {
		t.Error("downloaded content mismatch")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/transfers/"+id, nil)
	w = httptest.NewRecorder()
	srv.transferHandler(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("delete expected 204, got %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/transfers/"+id, nil)
	w = httptest.NewRecorder()
	srv.transferHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("after delete, expected 404, got %d", w.Code)
	}
}

func TestServer_TransferNotFound(t *testing.T) {
	srv := newTestServer(t)
	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/transfers/nonexistent"},
		{http.MethodGet, "/api/transfers/nonexistent/download"},
		{http.MethodDelete, "/api/transfers/nonexistent"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		srv.transferHandler(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s %s: expected 404, got %d", tc.method, tc.path, w.Code)
		}
	}
}

func TestServer_DownloadNotCompleted(t *testing.T) {
	srv := newTestServer(t)
	mgr := srv.mgr
	t1, _ := mgr.CreateTransfer("pending.txt", 100)
	req := httptest.NewRequest(http.MethodGet, "/api/transfers/"+t1.ID+"/download", nil)
	w := httptest.NewRecorder()
	srv.transferHandler(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestServer_UploadMissingFile(t *testing.T) {
	srv := newTestServer(t)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("notfile", "data")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.uploadHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func newTestServer(t *testing.T) *Server {
	tmp := t.TempDir()
	cfg := &config.Config{
		Host:    "127.0.0.1",
		Port:    0,
		TempDir: tmp,
	}
	log := logger.Default()
	mgr, err := transfer.NewManager(tmp, log)
	if err != nil {
		t.Fatal(err)
	}
	
	srv := New(cfg, mgr, log)
	return srv
}
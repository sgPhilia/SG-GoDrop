package main

import (
	"bytes"
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
	"godrop/internal/server"
	"godrop/internal/transfer"
)

// Integration test: run a full server, upload and download a file.
func TestIntegration_UploadDownload(t *testing.T) {
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
	defer mgr.Cleanup()

	srv := server.New(cfg, mgr, log)

	// Start a test HTTP server
	testServer := httptest.NewServer(srv.server.Handler)
	defer testServer.Close()

	// 1. Upload
	fileContent := []byte("integration test content")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "integ.txt")
	part.Write(fileContent)
	writer.Close()

	resp, err := http.Post(testServer.URL+"/api/upload", writer.FormDataContentType(), body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload failed: %d", resp.StatusCode)
	}
	// parse id
	var uploadResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		t.Fatal(err)
	}
	id, ok := uploadResp["id"].(string)
	if !ok || id == "" {
		t.Fatal("no id in response")
	}

	// 2. Wait for completion (or immediately since upload is synchronous)
	// 3. Download
	dlResp, err := http.Get(testServer.URL + "/api/transfers/" + id + "/download")
	if err != nil {
		t.Fatal(err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download failed: %d", dlResp.StatusCode)
	}
	downloaded, err := io.ReadAll(dlResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(downloaded, fileContent) {
		t.Error("content mismatch")
	}

	// 4. Delete
	req, _ := http.NewRequest(http.MethodDelete, testServer.URL+"/api/transfers/"+id, nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("delete expected 204, got %d", delResp.StatusCode)
	}
}
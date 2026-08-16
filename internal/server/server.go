package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/skip2/go-qrcode"

	"godrop/internal/config"
	"godrop/internal/server/web"
	"godrop/internal/transfer"
)

type Server struct {
	cfg    *config.Config
	mgr    *transfer.Manager
	log    *slog.Logger
	server *http.Server
	lanURL string
}

func New(cfg *config.Config, mgr *transfer.Manager, log *slog.Logger) *Server {
	s := &Server{
		cfg: cfg,
		mgr: mgr,
		log: log,
	}

	webContent := web.FS()

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.FS(webContent)))

	mux.HandleFunc("/api/upload", s.uploadHandler)
	mux.HandleFunc("/api/transfers", s.transfersHandler)
	mux.HandleFunc("/api/transfers/", s.transferHandler)
	mux.HandleFunc("/api/info", s.infoHandler)
	mux.HandleFunc("/api/qr", s.qrHandler)

	s.server = &http.Server{
		Addr:    net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler: mux,
	}

	return s
}

func (s *Server) Start(ctx context.Context) error {
	addrs := s.getLocalAddresses()

	if len(addrs) > 0 {
		s.lanURL = fmt.Sprintf(
			"http://%s:%d",
			addrs[0],
			s.cfg.Port,
		)
	}

	s.log.Info("GoDrop is running!")
	s.log.Info(fmt.Sprintf(
		"Local: http://localhost:%d",
		s.cfg.Port,
	))

	for _, addr := range addrs {
		s.log.Info(fmt.Sprintf(
			"LAN: http://%s:%d",
			addr,
			s.cfg.Port,
		))
	}

	s.log.Info("Waiting for connections...")

	err := s.server.ListenAndServe()

	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) getLocalAddresses() []string {
	addrs := []string{}

	ifaces, err := net.Interfaces()
	if err != nil {
		return addrs
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrsI, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrsI {
			if ipnet, ok := a.(*net.IPNet); ok &&
				!ipnet.IP.IsLoopback() &&
				ipnet.IP.To4() != nil {

				addrs = append(addrs, ipnet.IP.String())
			}
		}
	}

	return addrs
}

func (s *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		s.log.Error(
			"failed to parse multipart form",
			"error",
			err,
		)

		http.Error(
			w,
			"Invalid request",
			http.StatusBadRequest,
		)

		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.log.Error(
			"failed to get file from form",
			"error",
			err,
		)

		http.Error(
			w,
			"Missing file field",
			http.StatusBadRequest,
		)

		return
	}

	defer file.Close()

	t, err := s.mgr.CreateTransfer(
		header.Filename,
		header.Size,
	)
	if err != nil {
		s.log.Error(
			"failed to create transfer",
			"error",
			err,
		)

		http.Error(
			w,
			"Internal error",
			http.StatusInternalServerError,
		)

		return
	}

	written, err := s.mgr.SaveUploadedFile(
		t.ID,
		file,
	)
	if err != nil {
		s.log.Error(
			"upload failed",
			"id",
			t.ID,
			"error",
			err,
		)

		http.Error(
			w,
			"Upload failed",
			http.StatusInternalServerError,
		)

		return
	}

	s.log.Info(
		"upload completed",
		"id",
		t.ID,
		"name",
		t.Name,
		"size",
		written,
	)

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":   t.ID,
		"name": t.Name,
		"size": written,
	})
}

func (s *Server) transfersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	transfers := s.mgr.ListTransfers()

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"transfers": transfers,
	})
}

func (s *Server) transferHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(
		r.URL.Path,
		"/api/transfers/",
	)

	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if id == "" {
		http.Error(
			w,
			"Missing transfer ID",
			http.StatusBadRequest,
		)

		return
	}

	switch r.Method {
	case http.MethodGet:

		if len(parts) == 2 && parts[1] == "download" {
			s.downloadHandler(w, r, id)
			return
		}

		t, ok := s.mgr.GetTransfer(id)
		if !ok {
			http.Error(
				w,
				"Transfer not found",
				http.StatusNotFound,
			)

			return
		}

		w.Header().Set(
			"Content-Type",
			"application/json",
		)

		_ = json.NewEncoder(w).Encode(t)

	case http.MethodDelete:

		err := s.mgr.DeleteTransfer(id)
		if err != nil {
			http.Error(
				w,
				err.Error(),
				http.StatusNotFound,
			)

			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
	}
}

func (s *Server) downloadHandler(
	w http.ResponseWriter,
	r *http.Request,
	id string,
) {
	t, ok := s.mgr.GetTransfer(id)
	if !ok {
		http.Error(
			w,
			"Transfer not found",
			http.StatusNotFound,
		)

		return
	}

	if t.Status != transfer.StatusCompleted {
		http.Error(
			w,
			"Transfer not ready for download",
			http.StatusConflict,
		)

		return
	}

	f, err := os.Open(t.Path)
	if err != nil {
		s.log.Error(
			"failed to open file for download",
			"id",
			id,
			"error",
			err,
		)

		http.Error(
			w,
			"File not accessible",
			http.StatusInternalServerError,
		)

		return
	}

	defer f.Close()

	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(
			`attachment; filename="%s"`,
			t.Name,
		),
	)

	w.Header().Set(
		"Content-Type",
		"application/octet-stream",
	)

	w.Header().Set(
		"Content-Length",
		strconv.FormatInt(t.Size, 10),
	)

	_, err = io.Copy(w, f)
	if err != nil {
		s.log.Error(
			"download failed",
			"id",
			id,
			"error",
			err,
		)
	}
}

func (s *Server) infoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	url := s.lanURL

	if url == "" {
		url = fmt.Sprintf(
			"http://localhost:%d",
			s.cfg.Port,
		)
	}

	resp := map[string]interface{}{
		"name": "GoDrop",
		"url":  url,
		"host": s.cfg.Host,
		"port": s.cfg.Port,
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) qrHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	url := s.lanURL

	if url == "" {
		url = fmt.Sprintf(
			"http://localhost:%d",
			s.cfg.Port,
		)
	}

	png, err := qrcode.Encode(
		url,
		qrcode.Medium,
		256,
	)

	if err != nil {
		s.log.Error(
			"failed to generate QR code",
			"error",
			err,
		)

		http.Error(
			w,
			"Internal error",
			http.StatusInternalServerError,
		)

		return
	}

	w.Header().Set(
		"Content-Type",
		"image/png",
	)

	_, _ = w.Write(png)
}
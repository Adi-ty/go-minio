package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
)

type Handler struct {
	Router        *mux.Router
	MinioService   MinioService
	Server        *http.Server
}

type MinioService interface {
	UploadFile(ctx context.Context, bucketName, filePath string) (string, error)
	GetFile(ctx context.Context, bucketName, objectName, filePath string) error
	DeleteObject(ctx context.Context, bucketName, objectName string) error
}

type UploadRequest struct {
	BucketName string `json:"bucket_name"`
	FilePath   string `json:"file_path"`
}

type GetFileRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectName string `json:"object_name"`
	FilePath   string `json:"file_path"`
}

type DeleteObjectRequest struct {
    BucketName string `json:"bucket_name"`
    ObjectName string `json:"object_name"`
}

type Response struct {
    Message string `json:"message"`
}

func NewHandler(minioService MinioService) *Handler {
	h := &Handler{
		MinioService: minioService,
	}
	h.Router = mux.NewRouter()
	h.mapRoutes()
	h.Router.Use(JSONMiddleware)
	h.Router.Use(LoggingMiddleware)
	h.Router.Use(TimeoutMiddleware)

	h.Server = &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: h.Router,
	}

	return h
}

func (h *Handler) mapRoutes() {
	h.Router.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Everything is fine"))
	})

	h.Router.HandleFunc("/api/v1/upload", h.UploadFile).Methods("POST")
	h.Router.HandleFunc("/api/v1/get", h.GetFile).Methods("POST")
	h.Router.HandleFunc("/api/v1/delete", h.DeleteObject).Methods("POST")
}

func (h *Handler) Serve() error {
	go func() {
		if err := h.Server.ListenAndServe(); err != nil {
			log.Println(err.Error())
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	h.Server.Shutdown(ctx)

	log.Println("shut down gracefully")
	return nil
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	fileName, err := h.MinioService.UploadFile(r.Context(), req.BucketName, req.FilePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{"message": "file uploaded successfully", "file_name": fileName}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
    var req GetFileRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    err := h.MinioService.GetFile(r.Context(), req.BucketName, req.ObjectName, req.FilePath)
    if err != nil {
        http.Error(w, "Failed to get file", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(Response{Message: "File downloaded successfully"})
}

func (h *Handler) DeleteObject(w http.ResponseWriter, r *http.Request) {
	var req DeleteObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.MinioService.DeleteObject(r.Context(), req.BucketName, req.ObjectName)
	if err != nil {
		http.Error(w, "Failed to delete object", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Message: "Object deleted successfully"})
}
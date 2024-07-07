package http

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	Router         *mux.Router
	StorageService StorageService
	Server         *http.Server
}

func NewHandler(storageService StorageService) *Handler {
	h := &Handler{
		StorageService: storageService,
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
	h.Router.HandleFunc("/api/v1/check", h.CheckObject).Methods("POST")
	h.Router.HandleFunc("/api/v1/rename", h.RenameObject).Methods("POST")
	h.Router.HandleFunc("/api/v1/upload/multiple", h.UploadMultipleFiles).Methods("POST")
	h.Router.HandleFunc("/api/v1/get/multiple", h.GetMultipleFiles).Methods("POST")
	h.Router.HandleFunc("/api/v1/delete/multiple", h.DeleteMultipleFiles).Methods("POST")
	h.Router.HandleFunc("/api/v1/url", h.UploadPreSignedURL).Methods("POST")
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

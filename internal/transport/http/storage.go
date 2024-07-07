package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type StorageService interface {
	UploadFile(ctx context.Context, bucketName, objectName, filePath string) (string, error)
	GetFile(ctx context.Context, bucketName, objectName, filePath string) error
	DeleteObject(ctx context.Context, bucketName, objectName string) error
	CheckObject(ctx context.Context, bucketName, objectName string) (bool, error)
	RenameObject(ctx context.Context, bucketName, oldObjectName, newObjectName string) error
	DeleteMultipleObjects(ctx context.Context, bucketName string, objectNames []string) error
	UploadS3File(ctx context.Context, bucketName, objectName, filePath string) (string, error)
	GetS3File(bucketName, objectName, filePath string) error
	UploadPreSignedURL(ctx context.Context, bucketName, objectName string, expiry int64) (string, error)
	DeleteS3Object(ctx context.Context, bucketName, objectName string) (bool, error)
	DeleteMultipleS3Objects(bucketName string, objectNames []string) error
	// UploadGCSFile(ctx context.Context, bucketName, filePath string) (string, error)
}

type UploadRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectName string `json:"object_name"`
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

type CheckObjectRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectName string `json:"object_name"`
}

type RenameObjectRequest struct {
	BucketName    string `json:"bucket_name"`
	OldObjectName string `json:"old_object_name"`
	NewObjectName string `json:"new_object_name"`
}

type UploadFilesRequest struct {
	BucketName string            `json:"bucket_name"`
	FilePaths  map[string]string `json:"file_paths"` // key: object name, value: file path
}

type GetFilesRequest struct {
	BucketName string            `json:"bucket_name"`
	FilePaths  map[string]string `json:"file_paths"` // key: object name, value: file path
}

type DeleteMultipleObjects struct {
	BucketName  string   `json:"bucket_name"`
	ObjectNames []string `json:"object_names"`
}

type UploadPreSignedURLRequest struct {
	BucketName string `json:"bucket_name"`
	ObjectName string `json:"object_name"`
	Expiry     int64  `json:"expiry"`
}

type Response struct {
	Message string `json:"message"`
}

type CheckObjectResponse struct {
	Exists bool `json:"exists"`
}

type UploadPresignedURLResponse struct {
	URL string `json:"url"`
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	err := godotenv.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	cloudEnv := os.Getenv("CLOUD_ENV")

	if cloudEnv == "s3" {
		log.Info("Trying to upload to s3...")
		fileName, err := h.StorageService.UploadS3File(r.Context(), req.BucketName, req.ObjectName, req.FilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]string{"message": "file uploaded successfully", "file_name": fileName}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
		// } else if cloudEnv == "gcs" {
		// 	fileName, err := h.StorageService.UploadGCSFile(r.Context(), req.BucketName, req.FilePath)
		// 	if err != nil {
		// 		http.Error(w, err.Error(), http.StatusInternalServerError)
		// return
		// 	}

		// 	resp := map[string]string{"message": "file uploaded successfully", "file_name": fileName}
		// 	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// 		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		// return
		// 	}
	} else {
		fileName, err := h.StorageService.UploadFile(r.Context(), req.BucketName, req.ObjectName, req.FilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]string{"message": "file uploaded successfully", "file_name": fileName}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}
}

func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	var req GetFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := godotenv.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	cloudEnv := os.Getenv("CLOUD_ENV")

	if cloudEnv == "s3" {
		log.Info("trying to get from s3...")
		err := h.StorageService.GetS3File(req.BucketName, req.ObjectName, req.FilePath)
		if err != nil {
			http.Error(w, "Failed to get file", http.StatusInternalServerError)
			return
		}
	} else {
		err := h.StorageService.GetFile(r.Context(), req.BucketName, req.ObjectName, req.FilePath)
		if err != nil {
			http.Error(w, "Failed to get file", http.StatusInternalServerError)
			return
		}
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

	err := godotenv.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	cloudEnv := os.Getenv("CLOUD_ENV")

	if cloudEnv == "s3" {
		log.Info("Trying to delete from s3...")
		success, err := h.StorageService.DeleteS3Object(r.Context(), req.BucketName, req.ObjectName)
		if err != nil {
			http.Error(w, "Failed to delete object", http.StatusInternalServerError)
			return
		}
		if !success {
			http.Error(w, "failed to delete object", http.StatusInternalServerError)
		}
	} else {
		err := h.StorageService.DeleteObject(r.Context(), req.BucketName, req.ObjectName)
		if err != nil {
			http.Error(w, "Failed to delete object", http.StatusInternalServerError)
			return
		}
	}

	err = h.StorageService.DeleteObject(r.Context(), req.BucketName, req.ObjectName)
	if err != nil {
		http.Error(w, "Failed to delete object", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{Message: "Object deleted successfully"})
}

func (h *Handler) CheckObject(w http.ResponseWriter, r *http.Request) {
	var req CheckObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	exists, err := h.StorageService.CheckObject(r.Context(), req.BucketName, req.ObjectName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := CheckObjectResponse{Exists: exists}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) RenameObject(w http.ResponseWriter, r *http.Request) {
	var req RenameObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	err := h.StorageService.RenameObject(r.Context(), req.BucketName, req.OldObjectName, req.NewObjectName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{"message": "File renamed successfully"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) UploadMultipleFiles(w http.ResponseWriter, r *http.Request) {
	var req UploadFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var failedFiles []string
	for objectName, filePath := range req.FilePaths {
		if _, err := h.StorageService.UploadFile(r.Context(), req.BucketName, objectName, filePath); err != nil {
			failedFiles = append(failedFiles, filePath)
		}
	}

	if len(failedFiles) > 0 {
		resp := map[string]interface{}{
			"message":      "Some files failed to upload",
			"failed_files": failedFiles,
		}
		w.WriteHeader(http.StatusPartialContent)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	resp := map[string]string{"message": "All files uploaded successfully"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) GetMultipleFiles(w http.ResponseWriter, r *http.Request) {
	var req GetFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var failedFiles []string
	for objectName, localFilePath := range req.FilePaths {
		if err := h.StorageService.GetFile(r.Context(), req.BucketName, objectName, localFilePath); err != nil {
			failedFiles = append(failedFiles, objectName)
		}
	}

	if len(failedFiles) > 0 {
		resp := map[string]interface{}{
			"message":      "Some files failed to download",
			"failed_files": failedFiles,
		}
		w.WriteHeader(http.StatusPartialContent)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	resp := map[string]string{"message": "All files downloaded successfully"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) DeleteMultipleFiles(w http.ResponseWriter, r *http.Request) {
	var req DeleteMultipleObjects
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	if err := h.StorageService.DeleteMultipleObjects(r.Context(), req.BucketName, req.ObjectNames); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := map[string]string{"message": "files deleted successfully"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) UploadPreSignedURL(w http.ResponseWriter, r *http.Request) {
	var req UploadPreSignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	url, err := h.StorageService.UploadPreSignedURL(r.Context(), req.BucketName, req.ObjectName, req.Expiry)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := UploadPresignedURLResponse{URL: url}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

package minio

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Service struct {
	MinioClient *minio.Client
}

func NewService() *Service {
	endpoint := "localhost:9000"
	accessKeyID := "user"
	secretAccessKey := "secret-key"
	useSSL := false

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("Failed to initialize MinIO client: %v", err)
	}

	return &Service{
		MinioClient: minioClient,
	}
}

func (s *Service) UploadFile(ctx context.Context, bucketName, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	objectName := filepath.Base(filePath)
	contentType := "application/octet-stream"

	fileStat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	_, err = s.MinioClient.PutObject(context.Background(), bucketName, objectName, file, fileStat.Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return objectName, nil
}

func (s *Service) GetFile(ctx context.Context, bucketName, objectName, filePath string) error {
	localFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	obj, err := s.MinioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
    if err != nil {
        return err
    }
	defer obj.Close()

	if _, err := io.Copy(localFile, obj); err != nil {
		return err
	}

	return nil
}

func (s *Service) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	err := s.MinioClient.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) CheckObject(ctx context.Context, bucketName, objectName string) (bool, error) {
	_, err := s.MinioClient.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *Service) RenameObject(ctx context.Context, bucketName, oldObjectName, newObjectName string) error {
	src := minio.CopySrcOptions{
		Bucket: bucketName,
        Object: oldObjectName,
	}
	dst := minio.CopyDestOptions{
        Bucket: bucketName,
        Object: newObjectName,
    }

	_, err := s.MinioClient.CopyObject(ctx, dst, src)
    if err != nil {
        return err
    }

	err = s.MinioClient.RemoveObject(ctx, bucketName, oldObjectName, minio.RemoveObjectOptions{})
    if err != nil {
        return err
    }

    return nil
}

func (s *Service) DeleteMultipleObjects(ctx context.Context, bucketName string, objectNames []string) error {
	objectsCh := make(chan string)

	go func() {
		defer close(objectsCh)
		for _, objectName := range objectNames {
			objectsCh <- objectName
		}
	}()

	removeObjectsOptions := minio.RemoveObjectsOptions{}
	// Convert objectsCh to <-chan minio.ObjectInfo
	objectInfoCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectInfoCh)
		for objectName := range objectsCh {
			objectInfoCh <- minio.ObjectInfo{Key: objectName}
		}
	}()

	for rErr := range s.MinioClient.RemoveObjects(ctx, bucketName, objectInfoCh, removeObjectsOptions) {
		if rErr.Err != nil {
			return rErr.Err
		}
	}

	return nil
}
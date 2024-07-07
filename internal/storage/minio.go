package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
)

func (s *Service) getClient(bucketName string) (*minio.Client, error) {
	client, exists := s.MinioClients[bucketName]
	if !exists {
		return nil, fmt.Errorf("no client found for bucket %s", bucketName)
	}
	return client, nil
}

func (s *Service) UploadFile(ctx context.Context, bucketName, objectName, filePath string) (string, error) {
	client, err := s.getClient(bucketName)
	if err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	contentType := "application/octet-stream"

	fileStat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	_, err = client.PutObject(context.Background(), bucketName, objectName, file, fileStat.Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return objectName, nil
}

func (s *Service) GetFile(ctx context.Context, bucketName, objectName, filePath string) error {
	client, err := s.getClient(bucketName)
	if err != nil {
		log.Fatalf("unable to find minio client: %s", err)
		return err
	}

	desDir := filepath.Dir(filePath)
	err = os.MkdirAll(desDir, os.ModePerm)
	if err != nil {
		log.Fatalf("failed to create destination directory: %s", err)
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	localFile, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed to create file: %s", err)
		return fmt.Errorf("failed to create file: %w", err)
	}

	obj, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		log.Fatalf("minio get object method failed: %s", err)
		return err
	}
	defer obj.Close()

	if _, err := io.Copy(localFile, obj); err != nil {
		return err
	}

	return nil
}

func (s *Service) DeleteObject(ctx context.Context, bucketName, objectName string) error {
	client, err := s.getClient(bucketName)
	if err != nil {
		return err
	}

	err = client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) CheckObject(ctx context.Context, bucketName, objectName string) (bool, error) {
	client, err := s.getClient(bucketName)
	if err != nil {
		return false, err
	}

	_, err = client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if err.Error() == "The specified key does not exist." {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (s *Service) RenameObject(ctx context.Context, bucketName, oldObjectName, newObjectName string) error {
	client, err := s.getClient(bucketName)
	if err != nil {
		return err
	}

	src := minio.CopySrcOptions{
		Bucket: bucketName,
		Object: oldObjectName,
	}
	dst := minio.CopyDestOptions{
		Bucket: bucketName,
		Object: newObjectName,
	}

	_, err = client.CopyObject(ctx, dst, src)
	if err != nil {
		return err
	}

	err = client.RemoveObject(ctx, bucketName, oldObjectName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) DeleteMultipleObjects(ctx context.Context, bucketName string, objectNames []string) error {
	client, err := s.getClient(bucketName)
	if err != nil {
		return err
	}

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

	for rErr := range client.RemoveObjects(ctx, bucketName, objectInfoCh, removeObjectsOptions) {
		if rErr.Err != nil {
			return rErr.Err
		}
	}

	return nil
}

func (s *Service) UploadPreSignedURL(ctx context.Context, bucketName, objectName string, expiry int64) (string, error) {
	client, err := s.getClient(bucketName)
	if err != nil {
		return "", err
	}

	expiryTime := time.Duration(expiry) * time.Second
	url, err := client.PresignedPutObject(ctx, bucketName, objectName, expiryTime)
	if err != nil {
		return "", err
	}

	return url.String(), nil
}

package minio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Service struct {
	MinioClients map[string]*minio.Client
}

type MinioBucket struct {
	Name            string `json:"name"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
}

type Config struct {
	Endpoint string        `json:"endpoint"`
	Buckets  []MinioBucket `json:"buckets"`
}

func NewService() *Service {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	minioClients := make(map[string]*minio.Client)

	configJSON := os.Getenv("CREDENTIALS")
	if configJSON == "" {
		log.Fatalf("Credentials is not set in .env")
	}

	var config Config
	err = json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		log.Fatalf("Failed to parse credentials")
	}

	for _, bucket := range config.Buckets {
		useSSL := false

		client, err := minio.New(config.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(bucket.AccessKeyID, bucket.SecretAccessKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			log.Fatalf("Failed to initialize MinIO client for bucket %s: %v", bucket.Name, err)
		}
		minioClients[bucket.Name] = client
	}

	return &Service{
		MinioClients: minioClients,
	}
}

func (s *Service) getClient(bucketName string) (*minio.Client, error) {
	client, exists := s.MinioClients[bucketName]
	if !exists {
		return nil, fmt.Errorf("no client found for bucket %s", bucketName)
	}
	return client, nil
}

func (s *Service) UploadFile(ctx context.Context, bucketName, filePath string) (string, error) {
	client, err := s.getClient(bucketName)
	if err != nil {
		return "", err
	}

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

	_, err = client.PutObject(context.Background(), bucketName, objectName, file, fileStat.Size(), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return objectName, nil
}

func (s *Service) GetFile(ctx context.Context, bucketName, objectName, filePath string) error {
	client, err := s.getClient(bucketName)
	if err != nil {
		return err
	}

	localFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	obj, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
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

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	// gcs "cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go/aws"
	s3Creds "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	// "golang.org/x/tools/go/analysis/passes/defers"
	// "google.golang.org/api/option"
)

type Service struct {
	MinioClients map[string]*minio.Client
	S3Clients    map[string]*s3.S3
	// GCSClients   map[string]*gcs.Client
}

type MinioBucket struct {
	Name            string `json:"name"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
}

type S3Bucket struct {
	Name            string `json:"name"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
}

type GCSBucket struct {
	Name string `json:"name"`
}

type Config struct {
	Minio struct {
		Endpoint string        `json:"endpoint"`
		Buckets  []MinioBucket `json:"buckets"`
	} `json:"minio"`
	S3 struct {
		Region  string     `json:"region"`
		Buckets []S3Bucket `json:"buckets"`
	} `json:"s3"`
	Gcs struct {
		ProjectID       string      `json:"projectID"`
		CredentialsFile string      `json:"credentialsFile"`
		Buckets         []GCSBucket `json:"buckets"`
	} `json:"gcs"`
}

func NewService() *Service {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	minioClients := make(map[string]*minio.Client)

	configJSON := os.Getenv("STORAGE_CREDENTIALS")
	if configJSON == "" {
		log.Fatalf("Credentials is not set in .env")
	}

	var config Config
	err = json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		log.Fatalf("Failed to parse credentials")
	}

	for _, bucket := range config.Minio.Buckets {
		useSSL := false

		client, err := minio.New(config.Minio.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(bucket.AccessKeyID, bucket.SecretAccessKey, ""),
			Secure: useSSL,
		})
		if err != nil {
			log.Fatalf("Failed to initialize MinIO client for bucket %s: %v", bucket.Name, err)
		}
		minioClients[bucket.Name] = client
	}

	s3Clients := make(map[string]*s3.S3)

	for _, bucket := range config.S3.Buckets {
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String(config.S3.Region),
			Credentials: s3Creds.NewStaticCredentials(
				bucket.AccessKeyID, bucket.SecretAccessKey, ""),
		})
		if err != nil {
			log.Fatalf("Failed to initialize s3 client for bucket")
		}
		s3Clients[bucket.Name] = s3.New(sess)
	}

	// gcsClients := make(map[string]*gcs.Client)

	// for _, bucket := range config.Gcs.Buckets {
	// 	client, err := gcs.NewClient(context.Background(), option.WithCredentialsFile(config.Gcs.CredentialsFile))
	// 	if err != nil {
	// 		log.Fatalf("Failed to initialize gcs client")
	// 	}
	// 	gcsClients[bucket.Name] = client
	// }

	return &Service{
		MinioClients: minioClients,
		S3Clients:    s3Clients,
		// GCSClients:   gcsClients,
	}
}

func (s *Service) getClient(bucketName string) (*minio.Client, error) {
	client, exists := s.MinioClients[bucketName]
	if !exists {
		return nil, fmt.Errorf("no client found for bucket %s", bucketName)
	}
	return client, nil
}

// func (s *Service) getS3Client(bucketName string) (*s3.S3, error) {
// 	client, exists := s.S3Clients[bucketName]
// 	if !exists {
// 		return nil, fmt.Errorf("no client found for bucket %s", bucketName)
// 	}
// 	return client, nil
// }

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

func (s *Service) UploadS3File(ctx context.Context, bucketName, filePath string) (string, error) {
	client, exists := s.S3Clients[bucketName]
	if !exists {
		return "", fmt.Errorf("no client found for bucket %s", bucketName)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	objectName := filepath.Base(filePath)

	// Create an uploader with the session and default options
	// uploader := s3manager.NewUploader(sess)
	// uploader := s3.New(session.Must(session.NewSession(&aws.Config{
	// 	Region: aws.String("ap-south-1"),
	// })))

	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to s3: %w", err)
	}

	return objectName, nil
}

func (s *Service) GetS3File(bucketName, objectName, filepath string) error {
	client, exists := s.S3Clients[bucketName]
	if !exists {
		return fmt.Errorf("no client found for %s", bucketName)
	}

	result, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return fmt.Errorf("couldn't get object %v:%v. Here's why: %v", bucketName, objectName, err)
	}
	defer result.Body.Close()

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("couldn't create file %v, as: %v", filepath, err)
	}
	defer file.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return fmt.Errorf("couldn't read the object %v, as %v", objectName, err)
	}

	_, err = file.Write(body)
	return err
}

// func (s *Service) UploadGCSFile(ctx context.Context, bucketName, filePath string) (string, error) {

// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to open file: %w", err)
// 	}
// 	defer file.Close()

// 	ObjectName := filepath.Base(filePath)
// 	contentType := "application/octet-stream"

// 	wc := s.GCSClients[bucketName].Bucket(bucketName).Object(ObjectName).NewWriter(ctx)
// 	defer wc.Close()

// 	wc.ContentType = contentType

// 	if _, err = io.Copy(wc, file); err != nil {
// 		return "", fmt.Errorf("failed to upload file: %w", err)
// 	}

// 	return ObjectName, nil
// }

func (s *Service) GetFile(ctx context.Context, bucketName, objectName, filePath string) error {
	client, err := s.getClient(bucketName)
	if err != nil {
		return err
	}

	desDir := filepath.Dir(filePath)
	err = os.Mkdir(desDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
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

func (s *Service) S3UploadFile(ctx context.Context, bucketName, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	objectName := filepath.Base(filePath)

	uploader := s3.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})))

	_, err = uploader.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return objectName, nil
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

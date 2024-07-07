package storage

import (
	"encoding/json"
	"os"

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
	log "github.com/sirupsen/logrus"
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

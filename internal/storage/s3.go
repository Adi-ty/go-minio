package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
)

func (s *Service) UploadS3File(ctx context.Context, bucketName, objectName, filePath string) (string, error) {
	client, exists := s.S3Clients[bucketName]
	if !exists {
		log.Fatalf("couldn't find client here")
		return "", fmt.Errorf("no s3 client found for bucket %s", bucketName)
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("failed to open file: %s", err)
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
		Body:   file,
	})
	if err != nil {
		log.Fatalf("failed to upload file to s3: %s", err)
		return "", fmt.Errorf("failed to upload file to s3: %w", err)
	}

	return objectName, nil
}

func (s *Service) GetS3File(bucketName, objectName, filePath string) error {
	client, exists := s.S3Clients[bucketName]
	if !exists {
		log.Fatalf("No s3 client found for %s", bucketName)
		return fmt.Errorf("no s3 client found for %s", bucketName)
	}

	result, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		log.Fatalf("couldn't get the object GetObject method failed: %s", err)
		return fmt.Errorf("couldn't get object %v:%v. Here's why: %v", bucketName, objectName, err)
	}
	defer result.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		log.Fatalf("failed create file at given filepath: %s", err)
		return fmt.Errorf("couldn't create file %v, as: %v", filePath, err)
	}
	defer file.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Fatalf("coudln't read the object: %s", err)
		return fmt.Errorf("couldn't read the object %v, as %v", objectName, err)
	}

	_, err = file.Write(body)
	return err
}

func (s *Service) DeleteS3Object(ctx context.Context, bucketName, objectName string) (bool, error) {
	client, exists := s.S3Clients[bucketName]
	if !exists {
		log.Fatalf("no client found for %s", bucketName)
		return false, fmt.Errorf("no s3 client found for %s", bucketName)
	}

	deleted := false
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}

	_, err := client.DeleteObject(input)
	if err != nil {
		log.Fatalf("failed to delete object: %s", err)
		return false, fmt.Errorf("failed to delete object: %w", err)
	} else {
		deleted = true
	}

	err = client.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		log.Fatalf("error occured: %s", err)
		return false, fmt.Errorf("error occurred while waiting for object to be deleted: %w", err)
	}

	return deleted, err
}

func (s *Service) DeleteMultipleS3Objects(bucketName string, objectNames []string) error {
	client, ok := s.S3Clients[bucketName]
	if !ok {
		log.Fatalf("no client found")
		return fmt.Errorf("no client found for bucket %s", bucketName)
	}

	objects := make([]*s3.ObjectIdentifier, len(objectNames))
	for i, objectName := range objectNames {
		objects[i] = &s3.ObjectIdentifier{Key: aws.String(objectName)}
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &s3.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	}

	_, err := client.DeleteObjects(input)
	if err != nil {
		log.Fatalf("failed to delete object: %s", err)
		return fmt.Errorf("failed to delete objects: %w", err)
	}

	return nil
}

func (s *Service) RenameS3Object(bucketName, oldObjectName, newObjectName string) error {
	client, exists := s.S3Clients[bucketName]
	if !exists {
		log.Fatalf("No s3 client found for %s", bucketName)
		return fmt.Errorf("no s3 client found for %s", bucketName)
	}

	_, err := client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		CopySource: aws.String(fmt.Sprintf("%v/%v", bucketName, oldObjectName)),
		Key:        aws.String(newObjectName),
	})
	if err != nil {
		log.Fatalf("failed to copy object: %s", err)
		return fmt.Errorf("failed to copy object: %w", err)
	}

	err = client.WaitUntilObjectExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(newObjectName),
	})
	if err != nil {
		log.Fatalf("error occured while waiting for object to be copied: %s", err)
		return fmt.Errorf("error occurred while waiting for object to be copied: %w", err)
	}

	_, err = client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(oldObjectName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete old object: %w", err)
	}

	err = client.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(oldObjectName),
	})
	if err != nil {
		return fmt.Errorf("error occurred while waiting for old object to be deleted: %w", err)
	}

	return nil
}

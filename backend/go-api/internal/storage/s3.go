package storage

import (
	"bytes"
	"context"
	"egobackend/internal/models"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Service struct {
	client *s3.S3
	bucket string
}

func NewS3Service(config models.S3Config) (*S3Service, error) {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.KeyID, config.AppKey, ""),
		Endpoint:         aws.String(config.Endpoint),
		Region:           aws.String(config.Region),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession, err := session.NewSession(s3Config)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать S3 сессию: %w", err)
	}

	s3Client := s3.New(newSession)

	return &S3Service{
		client: s3Client,
		bucket: config.Bucket,
	}, nil
}

func (s *S3Service) UploadFile(ctx context.Context, key string, mimeType string, data []byte) error {
	_, err := s.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return fmt.Errorf("не удалось загрузить файл в S3: %w", err)
	}
	log.Printf("[S3] Файл '%s' успешно загружен в бакет '%s'", key, s.bucket)
	return nil
}

func (s *S3Service) DeleteFiles(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	var objectsToDelete []*s3.ObjectIdentifier
	for _, key := range keys {
		objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
			Key: aws.String(key),
		})
	}

	_, err := s.client.DeleteObjectsWithContext(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &s3.Delete{
			Objects: objectsToDelete,
			Quiet:   aws.Bool(true),
		},
	})

	if err != nil {
		return fmt.Errorf("не удалось удалить файлы из S3: %w", err)
	}
	log.Printf("[S3] Успешно удалено %d объектов из бакета '%s'", len(keys), s.bucket)
	return nil
}

func (s *S3Service) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("не удалось получить объект %s из S3: %w", key, err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать тело объекта %s из S3: %w", key, err)
	}
	return body, nil
}

package utils

import (
	"errors"
	"go-novel/config"
	"log"
	"mime/multipart"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var s3Client *s3.S3

func InitS3() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			""),
		Endpoint: aws.String(cfg.S3Endpoint),
		Region:   aws.String("sgp1"),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		log.Fatalf("Failed to initialize new session: %v", err)
	}

	s3Client = s3.New(sess)
	return nil
}

func UploadFileToS3(file *multipart.FileHeader, folderName string) (*s3.PutObjectOutput, error) {
	if s3Client == nil {
		return nil, errors.New("s3Client is nil")
	}

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	key := folderName + "/" + file.Filename

	res, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String("webtales"),
		Key:    aws.String(key),
		Body:   src,
		ACL:    aws.String("public-read"),
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}

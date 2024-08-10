package utils

import (
	"fmt"
	"go-novel/config"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func DownloadAndUploadImage(imageURL, folderName string) (string, error) {

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	baseURL := getBaseURL(imageURL)
	req.Header.Set("Referer", baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	ext := filepath.Ext(imageURL)
	if ext == "" {

		ext = ".jpg"
	}

	tempFile, err := os.CreateTemp("", "novel-thumbnail-*"+ext)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("writing to temp file: %w", err)
	}

	_, err = tempFile.Seek(0, 0)
	if err != nil {
		return "", fmt.Errorf("resetting file pointer: %w", err)
	}

	s3URL, err := UploadFileToS3(tempFile.Name(), folderName, ext)
	if err != nil {
		return "", fmt.Errorf("uploading to S3: %w", err)
	}

	return s3URL, nil
}

func getBaseURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) >= 3 {
		return parts[0] + "//" + parts[2]
	}
	return url
}

func UploadFileToS3(filePath, folderName string, fileExt string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	fileName := filepath.Base(filePath)
	key := folderName + "/" + fileName

	mimeType := getMimeType(fileExt)

	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String("webtales"),
		Key:         aws.String(key),
		Body:        file,
		ACL:         aws.String("public-read"),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return "", fmt.Errorf("uploading to S3: %w", err)
	}

	s3URL := fmt.Sprintf("https://%s.%s/%s", "webtales", strings.TrimPrefix(s3Client.Endpoint, "https://"), key)

	return s3URL, nil
}

func getMimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

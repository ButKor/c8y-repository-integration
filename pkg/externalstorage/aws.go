package externalstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

type AWSClient struct {
	s3Client          *s3.Client
	s3PresignClient   *s3.PresignClient
	connectionDetails AwsConnectionDetails
	urlExpirationMins int
}

type AwsConnectionDetails struct {
	AccessKeyId     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	BucketName      string `json:"bucketName"`
	Region          string `json:"region"`
}

func (awsClient *AWSClient) Init(ctx context.Context, client *c8y.Client, tenantOptionCategory string, tenantOptionKey string, urlExpirationMins int) error {
	tenantOptionConnectionDetails, _, e := client.TenantOptions.GetOption(ctx, tenantOptionCategory, tenantOptionKey)
	if e != nil {
		slog.Error("AWS Credentials were not found in tenant options. Make sure a tenant option for category="+tenantOptionCategory+" and key="+tenantOptionKey+" exists and your service has READ access to tenant option. ", "err", e)
		return e
	}

	var connectionDetails AwsConnectionDetails
	err := json.Unmarshal([]byte(tenantOptionConnectionDetails.Value), &connectionDetails)
	if err != nil {
		slog.Error("Error while unmarshalling tenantOption for awsConnectionDetails. Make sure the tenantoptions value aligns with documentation.", "err", e)
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(connectionDetails.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(connectionDetails.AccessKeyId, connectionDetails.SecretAccessKey, "")))
	if err != nil {
		slog.Error("Error while loading default config for AWS connection.", "err", err)
		return err
	}
	c := s3.NewFromConfig(cfg)
	awsClient.s3Client = c
	awsClient.s3PresignClient = s3.NewPresignClient(c)
	awsClient.urlExpirationMins = urlExpirationMins
	awsClient.connectionDetails = connectionDetails
	return nil
}

func (awsClient *AWSClient) GetBucketName() string {
	return awsClient.connectionDetails.BucketName
}

func (awsClient *AWSClient) GetProviderName() string {
	return "aws"
}

func (awsClient *AWSClient) ListBucketContent() {
	output, err := awsClient.s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(awsClient.connectionDetails.BucketName),
	})
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("Bucket content:")
	for _, object := range output.Contents {
		slog.Info(fmt.Sprintf("key=%s size=%d", aws.ToString(object.Key), *object.Size))
	}
}

func (awsClient *AWSClient) GetPresignedURL(awsObjectKey string) (string, error) {
	presignedUrl, err := awsClient.s3PresignClient.PresignGetObject(context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(awsClient.connectionDetails.BucketName),
			Key:    aws.String(awsObjectKey),
		},
		s3.WithPresignExpires(time.Minute*time.Duration(awsClient.urlExpirationMins)))
	if err != nil {
		return "", err
	}
	return presignedUrl.URL, err
}

func (awsClient *AWSClient) GetFileContent(awsObjectKey string) (string, error) {
	result, err := awsClient.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(awsClient.connectionDetails.BucketName),
		Key:    aws.String(awsObjectKey),
	})

	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			slog.Warn("Can't get object from bucket. No such key existing", "awsObjectKey", awsObjectKey, "bucketName", awsClient.connectionDetails.BucketName)
		} else {
			slog.Warn("Couldn't get object from external storage", "awsObjectKey", awsObjectKey, "bucketName", awsClient.connectionDetails.BucketName, "err", err)
		}
		return "", err
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		slog.Warn("Couldn't read object from body", "awsObjectKey", awsObjectKey, "bucketName", awsClient.connectionDetails.BucketName, "err", err)
		return "", err
	}
	return string(body), nil
}

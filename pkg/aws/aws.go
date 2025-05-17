package aws

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
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
}

type AwsConnectionDetails struct {
	AccessKeyId     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	BucketName      string `json:"bucketName"`
	Region          string `json:"region"`
}

func NewClient(ctx context.Context, client *c8y.Client, tenantOptionCategory string, tenantOptionKey string) *AWSClient {
	// Read tenant option
	tenantOptionConnectionDetails, _, e := client.TenantOptions.GetOption(ctx, tenantOptionCategory, tenantOptionKey)
	if e != nil {
		slog.Error("AWS Credentials were not found in tenant options. Make sure a tenant option for category="+tenantOptionCategory+" and key="+tenantOptionKey+"awsConnectionDetails exists & your service has READ access to tenant option. Exiting now. ", "err", e)
		os.Exit(1)
	}

	// Parse value to struct
	var connectionDetails AwsConnectionDetails
	err := json.Unmarshal([]byte(tenantOptionConnectionDetails.Value), &connectionDetails)
	if err != nil {
		slog.Error("Error while unmarshalling tenantOption for awsConnectionDetails. Make sure the tenantoptions value aligns with documentation. Exiting now.", "err", e)
		os.Exit(1)
	}

	// Now create AWS Client
	cfg, _ := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(connectionDetails.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(connectionDetails.AccessKeyId, connectionDetails.SecretAccessKey, "")))
	c := s3.NewFromConfig(cfg)
	return &AWSClient{
		s3Client:          c,
		s3PresignClient:   s3.NewPresignClient(c),
		connectionDetails: connectionDetails,
	}
}

func (awsClient AWSClient) ListBucketContent() {
	output, err := awsClient.s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(awsClient.connectionDetails.BucketName),
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, object := range output.Contents {
		log.Printf("key=%s size=%d", aws.ToString(object.Key), *object.Size)
	}
}

func (awsClient AWSClient) GetPresignURL(awsObjectKey string) string {
	presignedUrl, err := awsClient.s3PresignClient.PresignGetObject(context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(awsClient.connectionDetails.BucketName),
			Key:    aws.String(awsObjectKey),
		},
		s3.WithPresignExpires(time.Minute*15))
	if err != nil {
		log.Fatal(err)
	}
	return presignedUrl.URL
}

func (awsClient AWSClient) GetFileContent(awsObjectKey string) (string, error) {
	result, err := awsClient.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(awsClient.connectionDetails.BucketName),
		Key:    aws.String(awsObjectKey),
	})

	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			log.Printf("Can't get object %s from bucket %s. No such key existing.\n", awsObjectKey, awsClient.connectionDetails.BucketName)
		} else {
			log.Printf("Couldn't get object %v:%v. Reason: %v\n", awsClient.connectionDetails.BucketName, awsObjectKey, err)
		}
		return "", err
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("Couldn't read object body from %v. Reason: %v\n", awsObjectKey, err)
		return "", err
	}
	return string(body), nil
}

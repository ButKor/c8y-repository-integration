package aws

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AwsCredentials struct {
	AccessId  string
	AccessKey string
}

func ListBucketContent(awsCfg aws.Config, awsBucketName string) {
	client := s3.NewFromConfig(awsCfg)
	output, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(awsBucketName),
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, object := range output.Contents {
		log.Printf("key=%s size=%d", aws.ToString(object.Key), *object.Size)
	}
}

func GetPresignURL(awsCfg aws.Config, awsBucketName string, awsObjectKey string) string {
	s3client := s3.NewFromConfig(awsCfg)
	presignClient := s3.NewPresignClient(s3client)
	presignedUrl, err := presignClient.PresignGetObject(context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(awsBucketName),
			Key:    aws.String(awsObjectKey),
		},
		s3.WithPresignExpires(time.Minute*15))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(presignedUrl.URL)
	return presignedUrl.URL
}

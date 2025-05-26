package externalstorage

import (
	"context"

	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

type ExternalStorageClient interface {
	Init(ctx context.Context, client *c8y.Client, tenantOptionCategory string, tenantOptionKey string, urlExpirationMins int) error
	GetFileContent(awsObjectKey string) (string, error)
	GetPresignedURL(awsObjectKey string) (string, error)
	ListBucketContent()
	GetBucketName() string
	GetProviderName() string
}

func ListBucketContent(esc ExternalStorageClient) {
	esc.ListBucketContent()
}

package externalstorage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	azblob "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"

	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

type AzClient struct {
	azBlobClient      *azblob.Client
	azContainerClient *container.Client
	ConnectionDetails AzConnectionDetails
	urlExpirationMins int
}

type AzConnectionDetails struct {
	ConnectionString string `json:"connectionString"`
	ContainerName    string `json:"containerName"`
}

func (azureClient *AzClient) Init(ctx context.Context, client *c8y.Client, tenantOptionCategory string, tenantOptionKey string, urlExpirationMins int) error {
	tenantOptionConnectionDetails, _, e := client.TenantOptions.GetOption(ctx, tenantOptionCategory, tenantOptionKey)
	if e != nil {
		slog.Error("Azure Credentials were not found in tenant options. Make sure a tenant option for category="+tenantOptionCategory+" and key="+tenantOptionKey+" exists and your service has READ access to tenant option. ", "err", e)
		return e
	}

	var connectionDetails AzConnectionDetails
	err := json.Unmarshal([]byte(tenantOptionConnectionDetails.Value), &connectionDetails)
	if err != nil {
		slog.Error("Error while unmarshalling tenantOption for azureConnectionDetails. Make sure the tenantoptions value aligns with documentation.", "err", e)
		return err
	}
	if len(connectionDetails.ConnectionString) == 0 {
		return errors.New("connection String could not be found in tenant option")
	}
	if len(connectionDetails.ContainerName) == 0 {
		return errors.New("container name could not be found in tenant option")
	}

	azureClient.ConnectionDetails = connectionDetails
	azureClient.urlExpirationMins = urlExpirationMins
	azureClient.azBlobClient, err = azblob.NewClientFromConnectionString(connectionDetails.ConnectionString, nil)
	if err != nil {
		slog.Error("Error while creating Azure Client from connection string. Make sure the connection string is set properly", "err", e)
		return err
	}
	azureClient.azContainerClient = azureClient.azBlobClient.ServiceClient().NewContainerClient(connectionDetails.ContainerName)
	return nil
}

func (azClient *AzClient) GetBucketName() string {
	return azClient.ConnectionDetails.ContainerName
}

func (azClient *AzClient) GetProviderName() string {
	return "azblob"
}

func (azClient *AzClient) ListBucketContent() {
	containerName := azClient.ConnectionDetails.ContainerName
	pager := azClient.azBlobClient.NewListBlobsFlatPager(containerName, &azblob.ListBlobsFlatOptions{
		Include: azblob.ListBlobsInclude{Snapshots: true, Versions: true},
	})
	slog.Info("Bucket content:")
	for pager.More() {
		resp, err := pager.NextPage(context.TODO())
		if err != nil {
			slog.Error("Error when paging trough container list", "err", err)
		}
		for _, blob := range resp.Segment.BlobItems {
			slog.Info(fmt.Sprintf("name=%s", *blob.Name))
		}
	}
}

func (azClient *AzClient) GetPresignedURL(azObjectFileName string) (string, error) {
	cc := azClient.azContainerClient
	start := time.Now()
	sasurl, err := cc.NewBlobClient(azObjectFileName).GetSASURL(
		sas.BlobPermissions{Read: true},
		start.Add(time.Minute*time.Duration(azClient.urlExpirationMins)),
		&blob.GetSASURLOptions{
			StartTime: &start,
		},
	)
	if err != nil {
		return "", err
	}
	return sasurl, nil
}

func (azClient *AzClient) GetFileContent(azObjectFileName string) (string, error) {
	get, err := azClient.azBlobClient.DownloadStream(context.TODO(), azClient.ConnectionDetails.ContainerName, azObjectFileName, nil)
	if err != nil {
		return "", err
	}

	downloadedData := bytes.Buffer{}
	retryReader := get.NewRetryReader(context.TODO(), &azblob.RetryReaderOptions{})
	if _, err = downloadedData.ReadFrom(retryReader); err != nil {
		return "", err
	}

	if err = retryReader.Close(); err != nil {
		return "", err
	}

	return downloadedData.String(), nil
}

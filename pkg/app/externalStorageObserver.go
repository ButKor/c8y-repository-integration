package app

import (
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/kobu/repo-int/pkg/aws"
)

type FirmwareIndexEntry struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	IsPatch         bool   `json:"isPatch"`
	PatchDependency string `json:"patchDependency"`
}

type ExternalStorageObserver struct {
	awsClient            *aws.AWSClient
	lastKnownHash        string
	firmwareIndexEntries []FirmwareIndexEntry
	tenantControllers    *FirmwareTenantControllers
}

func (o *ExternalStorageObserver) AutoObserve(syncIntervalSecs int64) {

}

func (o *ExternalStorageObserver) ReadIndexFile() string {
	fc, _ := os.ReadFile("firmware-index.json")
	return string(fc)
	// fc, _ := o.awsClient.GetFileContent("firmware-index.json")
	// return fc
}

func (o *ExternalStorageObserver) SyncIndexFile() {
	// Get file content of index file and parse it a
	fc := o.ReadIndexFile()
	firmwareIndexEntries, hash := ProcessIndexFileContents(fc)
	// input has changed, update lastKnownHash and notify all tenant controllers
	if o.lastKnownHash != hash {
		slog.Info("Changes in externals index file detected")
		o.firmwareIndexEntries = firmwareIndexEntries
		o.tenantControllers.NotifyExtIndexChanged(firmwareIndexEntries)
	}
	o.lastKnownHash = hash
}

// input = content of the index file located on external storage
// return = [List of parsed FimrwareIndexEntries] , [Hash of input string]
func ProcessIndexFileContents(fileContent string) ([]FirmwareIndexEntry, string) {
	h := sha256.New()
	h.Write([]byte(fileContent))
	hash := string(h.Sum(nil))

	var indexEntries []FirmwareIndexEntry
	for _, e := range strings.Split(string(fileContent), "\n") {
		data := FirmwareIndexEntry{}
		err := json.Unmarshal([]byte(e), &data)
		if err != nil {
			slog.Error("Error wile unmarshaling following line: " + e + ". Skipping this entry. Error: " + err.Error())
			continue
		}
		indexEntries = append(indexEntries, data)
	}
	return indexEntries, hash
}

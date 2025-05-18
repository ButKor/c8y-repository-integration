package app

import (
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/kobu/repo-int/pkg/aws"
)

type ExtFirmwareInfoEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DeviceType  string `json:"deviceType"`
}

type ExtFirmwareVersionEntry struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ExternalStorageObserver struct {
	awsClient             *aws.AWSClient
	lastKnownHashVersions string
	firmwareIndexEntries  []ExtFirmwareVersionEntry
	tenantControllers     *FirmwareTenantControllers
}

func (o *ExternalStorageObserver) AutoObserve(syncIntervalSecs int64) {

}

func (o *ExternalStorageObserver) ReadIndexFile() string {
	fc, _ := os.ReadFile("c8y-firmware-versions.json")
	return string(fc)
	// fc, _ := o.awsClient.GetFileContent("firmware-index.json")
	// return fc
}

func (o *ExternalStorageObserver) ReadFirmwareInfoFile() string {
	fc, _ := os.ReadFile("c8y-firmware-info.json")
	return string(fc)
}

func (o *ExternalStorageObserver) SyncFirmwareVersionsFile() {
	// Get file content of index file and parse it a
	fc := o.ReadIndexFile()
	firmwareIndexEntries, hash := ProcessExtFwVersionContents(fc)
	// input has changed, update lastKnownHash and notify all tenant controllers
	if o.lastKnownHashVersions != hash {
		slog.Info("Changes in externals index file detected")
		o.firmwareIndexEntries = firmwareIndexEntries
		fi := o.ReadFirmwareInfoFile()
		firmwareInfoEntries := ProcessFirmwareInfoContents(fi)
		o.tenantControllers.NotifyExtIndexChanged(firmwareIndexEntries, firmwareInfoEntries)
	}
	o.lastKnownHashVersions = hash
}

// input = content of the index file located on external storage
// return = [List of parsed FimrwareIndexEntries] , [Hash of input string]
func ProcessExtFwVersionContents(fileContent string) ([]ExtFirmwareVersionEntry, string) {
	h := sha256.New()
	h.Write([]byte(fileContent))
	hash := string(h.Sum(nil))

	var indexEntries []ExtFirmwareVersionEntry
	for _, e := range strings.Split(string(fileContent), "\n") {
		data := ExtFirmwareVersionEntry{}
		err := json.Unmarshal([]byte(e), &data)
		if err != nil {
			slog.Error("Error wile unmarshaling following line: " + e + ". Skipping this entry. Error: " + err.Error())
			continue
		}
		indexEntries = append(indexEntries, data)
	}
	return indexEntries, hash
}

func ProcessFirmwareInfoContents(fileContent string) map[string]ExtFirmwareInfoEntry {
	res := make(map[string]ExtFirmwareInfoEntry)
	for _, e := range strings.Split(string(fileContent), "\n") {
		data := ExtFirmwareInfoEntry{}
		err := json.Unmarshal([]byte(e), &data)
		if err != nil {
			slog.Error("Error wile unmarshaling following line: " + e + ". Skipping this entry. Error: " + err.Error())
			continue
		}
		res[data.Name] = data
	}
	return res
}

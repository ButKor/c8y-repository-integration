package app

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	est "github.com/kobu/dm-repo-integration/pkg/externalstorage"
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

type FirmwareTenantControllers struct {
	tenantControllers  map[string]FirmwareTenantController
	estClient          est.ExternalStorageClient
	lastKnownInputHash string
}

func (c *FirmwareTenantControllers) Register(fc FirmwareTenantController) {
	if c.tenantControllers == nil {
		c.tenantControllers = make(map[string]FirmwareTenantController, 1)
	}
	c.tenantControllers[fc.tenantId] = fc
}

func (c *FirmwareTenantControllers) Get(tenantId string) (FirmwareTenantController, bool) {
	val, ok := c.tenantControllers[tenantId]
	return val, ok
}

func (c *FirmwareTenantControllers) AutoObserve(intervalSecs int) {
	go func() {
		for {
			c.SyncAllRegisteredTenantsWithIndexFiles()
			time.Sleep(time.Duration(intervalSecs) * time.Second)
		}
	}()
}

func (c *FirmwareTenantControllers) SyncAllRegisteredTenantsWithIndexFiles() {
	c.SyncTenantsWithIndexFiles(slices.Collect(maps.Keys(c.tenantControllers)), false)
}

func (c *FirmwareTenantControllers) SyncTenantsWithIndexFiles(tenantIds []string, enforceSync bool) {
	contentFwVersionFile := c.ReadExtFileContentsAsString("c8y-firmware-versions.json")
	if len(contentFwVersionFile) == 0 {
		slog.Error("Firmware Version Info file (c8y-firmware-versions.json) could not be read or is empty. Service stops syncing attempt.")
		return
	}
	contentFwInfoFile := c.ReadExtFileContentsAsString("c8y-firmware-info.json")
	if len(contentFwInfoFile) == 0 {
		slog.Error("Firmware Info file (c8y-firmware-info.json) could not be read or is empty. Service stops syncing attempt.")
		return
	}
	inputHash := GetMD5Hash(contentFwVersionFile) + GetMD5Hash(contentFwVersionFile)

	if inputHash == c.lastKnownInputHash && !enforceSync {
		slog.Info("No change in input files, nothing to do here.", "inputHash", inputHash, "lastKnownHash", c.lastKnownInputHash, "enforceSync", enforceSync)
		return
	}

	slog.Info("Hashes for Input Files differing between current run and last run. Parsing the external files now")
	fwVersionEntries := ParseExtFwVersionContents(contentFwVersionFile)
	fwInfoEntries := ParseExtFwInfoContents(contentFwInfoFile)

	slog.Info("Now apply changes in each tenant")
	for _, e := range tenantIds {
		val, ok := c.tenantControllers[e]
		if !ok {
			slog.Warn("No Firmware Controller found for Tenant. Skipping this tenant.", "tenantId", e)
			continue
		}
		val.ExternalStorageIndexChanged(fwVersionEntries, fwInfoEntries, enforceSync)
	}
	c.lastKnownInputHash = inputHash
}

func (c *FirmwareTenantControllers) ReadExtFileContentsAsString(objectKey string) string {
	res, err := c.estClient.GetFileContent(objectKey)
	if err != nil {
		slog.Error("Error while reading file from external storage", "objectKey", objectKey, "err", err)
		return ""
	}
	return res
}

// input = content of the index file located on external storage
// return = [List of parsed FimrwareIndexEntries] , [Hash of input string]
func ParseExtFwVersionContents(fileContentFwVersionFile string) []ExtFirmwareVersionEntry {
	var indexEntries []ExtFirmwareVersionEntry
	for _, e := range strings.Split(string(fileContentFwVersionFile), "\n") {
		data := ExtFirmwareVersionEntry{}
		err := json.Unmarshal([]byte(e), &data)
		if err != nil {
			slog.Error("Error wile unmarshaling following line: "+e+". Skipping this entry", "err", err)
			continue
		}
		indexEntries = append(indexEntries, data)
	}
	return indexEntries
}

func ParseExtFwInfoContents(fileContentFwInfoFile string) map[string]ExtFirmwareInfoEntry {
	res := make(map[string]ExtFirmwareInfoEntry)
	for _, e := range strings.Split(string(fileContentFwInfoFile), "\n") {
		if strings.HasPrefix("#", e) || len(e) == 0 {
			continue
		}
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

func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

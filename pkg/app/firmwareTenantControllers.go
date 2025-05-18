package app

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
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
	tenantControllers map[string]FirmwareTenantController
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

func (c *FirmwareTenantControllers) SyncAllRegisteredTenantsWithIndexFiles() {
	c.SyncTenantsWithIndexFiles(slices.Collect(maps.Keys(c.tenantControllers)))
}

func (c *FirmwareTenantControllers) SyncTenantsWithIndexFiles(tenantIds []string) {
	// TODO: error and null check
	fwVersionEntries := ReadExtFwVersionContents()
	fwInfoEntries := ReadFirmwareInfoContents()
	for _, e := range tenantIds {
		val, ok := c.tenantControllers[e]
		if !ok {
			slog.Warn("No Firmware Controller found for Tenant. Skipping this tenant.", "tenantId", e)
			continue
		}
		val.ExternalStorageIndexChanged(fwVersionEntries, fwInfoEntries)
	}
}

// input = content of the index file located on external storage
// return = [List of parsed FimrwareIndexEntries] , [Hash of input string]
func ReadExtFwVersionContents() []ExtFirmwareVersionEntry {
	fc, _ := os.ReadFile("c8y-firmware-versions.json")
	// fc, _ := o.awsClient.GetFileContent("firmware-index.json")
	// return fc
	var indexEntries []ExtFirmwareVersionEntry
	for _, e := range strings.Split(string(fc), "\n") {
		data := ExtFirmwareVersionEntry{}
		err := json.Unmarshal([]byte(e), &data)
		if err != nil {
			slog.Error("Error wile unmarshaling following line: " + e + ". Skipping this entry. Error: " + err.Error())
			continue
		}
		indexEntries = append(indexEntries, data)
	}
	return indexEntries
}

func ReadFirmwareInfoContents() map[string]ExtFirmwareInfoEntry {
	fc, _ := os.ReadFile("c8y-firmware-info.json")
	// fc, _ := o.awsClient.GetFileContent("firmware-index.json")
	// return fc
	res := make(map[string]ExtFirmwareInfoEntry)
	for _, e := range strings.Split(string(fc), "\n") {
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

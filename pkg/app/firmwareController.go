package app

import (
	"context"
	"fmt"
	"log/slog"

	est "github.com/kobu/repo-int/pkg/externalstorage"
	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

type FirmwareTenantController struct {
	tenantId       string
	tenantStore    *FirmwareTenantStore
	ctx            context.Context
	c8yClient      *c8y.Client
	estClient      *est.ExternalStorageClient
	serviceBaseUrl string
}

type ExternalResourceOrigin struct {
	Provider   string `json:"provider,omitempty"`
	BucketName string `json:"container,omitempty"`
	ObjectKey  string `json:"objectKey,omitempty"`
	CreatedBy  string `json:"createdBy,omitempty"`
}

type C8yFirmware struct {
	Url     string `json:"url,omitempty"`
	Version string `json:"version,omitempty"`
}

type FirmwareVersion struct {
	c8y.ManagedObject
	C8yFirmware *C8yFirmware            `json:"c8y_Firmware"`
	Origin      *ExternalResourceOrigin `json:"externalResourceOrigin,omitempty"`
}

type C8yFilter struct {
	Type string `json:"type"`
}

type Firmware struct {
	c8y.ManagedObject
	Filter      *C8yFilter              `json:"c8y_Filter"`
	Description string                  `json:"description"`
	Origin      *ExternalResourceOrigin `json:"resourceOrigin,omitempty"`
}

func NewFirmware(name string, fwInfo ExtFirmwareInfoEntry, provider string, bucketName string, objectKey string) *Firmware {
	res := &Firmware{
		ManagedObject: c8y.ManagedObject{
			Name: name,
			Type: "c8y_Firmware",
		},
		Description: fwInfo.Description,
		Origin: &ExternalResourceOrigin{
			Provider:   provider,
			BucketName: bucketName,
			ObjectKey:  objectKey,
			CreatedBy:  "repository-integration-service",
		},
		Filter: &C8yFilter{Type: fwInfo.DeviceType},
	}
	return res
}

func NewFirmwareVersion(name string, version string, url string, provider string, bucketName string, objectKey string) FirmwareVersion {
	return FirmwareVersion{
		ManagedObject: c8y.ManagedObject{
			Name: name,
			Type: "c8y_FirmwareBinary",
		},
		C8yFirmware: &C8yFirmware{
			Url:     url,
			Version: version,
		},
		Origin: &ExternalResourceOrigin{
			Provider:   provider,
			BucketName: bucketName,
			ObjectKey:  objectKey,
		},
	}
}

func (c *FirmwareTenantController) ExternalStorageIndexChanged(extFwVersionEntries []ExtFirmwareVersionEntry, extFwInfoEntries map[string]ExtFirmwareInfoEntry) {
	c.rebuildTenantStore()
	syncExtFwVersionEntriesWithCumulocity(c, extFwVersionEntries, extFwInfoEntries)
	syncCumulocityWithextFwVersionEntries(c, extFwVersionEntries)
}

// run over the index entries (from ext. storage) and check if they are all existing. If no, create it in Cumulocity
func syncExtFwVersionEntriesWithCumulocity(controller *FirmwareTenantController, extFwVersionEntries []ExtFirmwareVersionEntry, extFwInfoEntries map[string]ExtFirmwareInfoEntry) {
	for _, extFwVersionEntry := range extFwVersionEntries {
		_, vok := controller.tenantStore.GetFirmwareVersion(extFwVersionEntry.Name, extFwVersionEntry.Version)
		if !vok {
			// version not in tenant store, is the firmware itself available?
			existingFirmware, fok := controller.tenantStore.GetFirmware(extFwVersionEntry.Name)
			if !fok {
				createdFirmwareMoId, fwCreateErr := createFirmware(controller, extFwVersionEntry, extFwInfoEntries[extFwVersionEntry.Name], true)
				if fwCreateErr != nil {
					slog.Error("Error while creating Firmware. Skipping this iteration.", "error", fwCreateErr.Error)
					continue
				}
				// create firmware version & assign to Firmware
				createAndReferenceFirmwareVersion(controller, createdFirmwareMoId, extFwVersionEntry.Name, extFwVersionEntry.Version, extFwVersionEntry.Key, true)
			} else {
				// firmware is already existing, add version object
				createAndReferenceFirmwareVersion(controller, existingFirmware.MoId, extFwVersionEntry.Name, extFwVersionEntry.Version, extFwVersionEntry.Key, true)
			}
		}
	}
}

func createFirmware(controller *FirmwareTenantController, extFwVersionEntry ExtFirmwareVersionEntry, extFwInfoEntry ExtFirmwareInfoEntry, updateTenantStore bool) (string, error) {
	estClient := *controller.estClient
	createdFirmware, _, fwErr := controller.c8yClient.Inventory.Create(controller.ctx,
		NewFirmware(extFwVersionEntry.Name, extFwInfoEntry, estClient.GetProviderName(), estClient.GetBucketName(), extFwVersionEntry.Key))
	if fwErr != nil {
		return "", fwErr
	}
	slog.Info("Created Firmware", "moId", createdFirmware.ID)
	if updateTenantStore {
		controller.tenantStore.AddFirmware(FirmwareStoreFwEntry{
			TenantId: controller.tenantId,
			MoId:     createdFirmware.ID,
			MoName:   createdFirmware.Name,
			MoType:   createdFirmware.Type,
		})
	}
	return createdFirmware.ID, nil
}

func createAndReferenceFirmwareVersion(controller *FirmwareTenantController, fwMoId string, name string, version string, objectKey string, updateTenantStore bool) {
	// Create firmware version object
	estClient := *controller.estClient
	createdFwVersion, _, fwCreateErr := controller.c8yClient.Inventory.Create(
		controller.ctx,
		NewFirmwareVersion(name, version, "http://to-be-provided.org", estClient.GetProviderName(), estClient.GetBucketName(), objectKey))
	if fwCreateErr != nil {
		slog.Error("Error while creating Firmware version. Skipping this iteration.", "error", fwCreateErr.Error())
		return
	}
	slog.Info("Created Firmware Version", "moId", createdFwVersion.ID)
	// Set Version URL now
	versionUrl := controller.serviceBaseUrl + "/firmware/download?id=" + createdFwVersion.ID
	_, _, updateErr := controller.c8yClient.Inventory.Update(controller.ctx, createdFwVersion.ID, &FirmwareVersion{
		C8yFirmware: &C8yFirmware{
			Url:     versionUrl,
			Version: version,
		},
	})
	if updateErr != nil {
		slog.Error("Error while updating URL for firmware version. ", "fwVersionId", createdFwVersion.ID, "error", updateErr.Error())
	}
	slog.Info("Updated Firmware URL", "fwVersionId", createdFwVersion.ID, "url", versionUrl)
	// assign firmware version to firmware
	_, _, assignErr := controller.c8yClient.Inventory.AddChildAddition(controller.ctx, fwMoId, createdFwVersion.ID)
	if assignErr != nil {
		slog.Error("Error while assigning firmware version to firmware.", "firmwareMoId", fwMoId, "firmwareVersionMoId", createdFwVersion.ID, "error", assignErr.Error())
	} else {
		slog.Info("Assigned Firmware Version to Firmware", "firmwareMoId", fwMoId, "firmwareVersionMoId", createdFwVersion.ID)
	}
	// Register in tenantstore
	if updateTenantStore {
		controller.tenantStore.AddFirmwareVersion(FirmwareStoreVersionEntry{
			TenantId:        controller.tenantId,
			MoId:            createdFwVersion.ID,
			MoType:          createdFwVersion.Type,
			FwName:          createdFwVersion.Name,
			IsPatch:         false,
			PatchDependency: "",
			Version:         version,
			URL:             versionUrl,
		})
	}
}

// run over tenant store and check if they all exist in extFwVersionEntries. Remove from Cumulocity if not.
func syncCumulocityWithextFwVersionEntries(controller *FirmwareTenantController, extFwVersionEntries []ExtFirmwareVersionEntry) {
	for _, versionList := range controller.tenantStore.FirmwareVersionsByName {
		for _, version := range versionList {
			if !contains(extFwVersionEntries, version) {
				// version existing in Cumulocity but not in indexList => delete in Cumulocity
			}
		}
	}
}

func contains(extFwVersionEntries []ExtFirmwareVersionEntry, storeEntry FirmwareStoreVersionEntry) bool {
	for _, e := range extFwVersionEntries {
		if e.Name == storeEntry.FwName && e.Version == storeEntry.Version {
			return true
		}
	}
	return false
}

// scans tenants firmware repository and caches it to tenant store
// TODO: adapt pagesizes
func (c *FirmwareTenantController) rebuildTenantStore() {
	tenantName := c.c8yClient.GetTenantName(c.ctx)
	// collect all firmware objects
	cp := 1
	for {
		firmwares, _, _ := c.c8yClient.Inventory.GetManagedObjects(
			c.ctx, &c8y.ManagedObjectOptions{
				Type: "c8y_Firmware",
				PaginationOptions: c8y.PaginationOptions{
					PageSize:       1,
					CurrentPage:    &cp,
					WithTotalPages: true,
				},
			},
		)

		// iterate over firmware items, collect child-additions and register everything in the store
		for _, fwObject := range firmwares.Items {
			firmwareId := fwObject.Get("id").String()
			firmwareName := fwObject.Get("name").String()

			icp := 1
			for {
				childAdditionReferences, resp, _ := c.c8yClient.Inventory.GetChildAdditions(c.ctx, firmwareId, &c8y.ManagedObjectOptions{
					PaginationOptions: c8y.PaginationOptions{
						PageSize:       1,
						CurrentPage:    &icp,
						WithTotalPages: true,
					},
					Query: "type eq c8y_FirmwareBinary",
				})
				c.tenantStore.AddFirmware(FirmwareStoreFwEntry{
					TenantId: tenantName,
					MoId:     firmwareId,
					MoName:   firmwareName,
					MoType:   fwObject.Get("type").String(),
				})

				for _, ref := range resp.JSON("references").Array() {
					isPatch := ref.Get("managedObject.c8y_Patch").Exists()
					patchDependency := ref.Get("managedObject.c8y_Patch.dependency").String()
					fw := FirmwareStoreVersionEntry{
						TenantId:        tenantName,
						MoId:            ref.Get("managedObject.id").String(),
						MoType:          ref.Get("managedObject.type").String(),
						FwName:          firmwareName,
						IsPatch:         isPatch,
						PatchDependency: patchDependency,
						Version:         ref.Get("managedObject.c8y_Firmware.version").String(),
						URL:             ref.Get("managedObject.c8y_Firmware.url").String(),
					}
					c.tenantStore.AddFirmwareVersion(fw)
				}
				if *childAdditionReferences.Statistics.TotalPages == *childAdditionReferences.Statistics.CurrentPage {
					break
				}
				icp++
			}
		}

		if *firmwares.Statistics.CurrentPage == *firmwares.Statistics.TotalPages {
			break
		}
		cp++
	}
	fmt.Println("test")
}

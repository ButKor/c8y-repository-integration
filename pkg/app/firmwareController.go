package app

import (
	"context"
	"fmt"

	"github.com/kobu/repo-int/pkg/aws"
	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

type FirmwareTenantController struct {
	tenantId    string
	tenantStore *FirmwareTenantStore
	ctx         context.Context
	c8yClient   *c8y.Client
	awsClient   *aws.AWSClient
}

func (c *FirmwareTenantController) ExternalStorageIndexChanged(indexEntries []FirmwareIndexEntry) {
	c.rebuildTenantStore()
	syncIndexEntriesWithCumulocity(c, indexEntries)
	syncCumulocityWithIndexEntries(c, indexEntries)
	fmt.Println("ok")
}

// run over the index entries (from ext. storage) and check if they are all existing. If no, create it in Cumulocity
func syncIndexEntriesWithCumulocity(controller *FirmwareTenantController, indexEntries []FirmwareIndexEntry) {
	for _, e := range indexEntries {
		_, vok := controller.tenantStore.GetFirmwareVersion(e.Name, e.Version)
		if !vok {
			// version not in tenant store, is the firmware itself available?
			fw, fok := controller.tenantStore.GetFirmware(e.Name)
			if !fok {
				// Firmware not existing at all => create a new one and add version to it
			} else {
				// Firmware exists => add version to this Firmware
				fmt.Println(fw.MoId)
			}

		}
	}
}

// run over tenant store and check if they all exist in indexEntries. Remove from Cumulocity if not.
func syncCumulocityWithIndexEntries(controller *FirmwareTenantController, indexEntries []FirmwareIndexEntry) {
	for _, versionList := range controller.tenantStore.FirmwareVersionsByName {
		for _, version := range versionList {
			if !contains(indexEntries, version) {
				// version existing in Cumulocity but not in indexList => delete in Cumulocity
			}
		}
	}
}

func contains(indexEntries []FirmwareIndexEntry, storeEntry FirmwareStoreVersionEntry) bool {
	for _, e := range indexEntries {
		if e.Name == storeEntry.Name && e.Version == storeEntry.Version {
			return true
		}
	}
	return false
}

// scans tenants firmware repository and caches it to tenant store
// TODO: support more than 2000 elements
func (c *FirmwareTenantController) rebuildTenantStore() {
	tenantName := c.c8yClient.GetTenantName(c.ctx)
	// collect all firmware objects
	firmwares, _, _ := c.c8yClient.Inventory.GetManagedObjects(
		c.ctx, &c8y.ManagedObjectOptions{
			Type:              "c8y_Firmware",
			PaginationOptions: *c8y.NewPaginationOptions(2000),
		},
	)

	// iterate over firmware items, collect child-additions and register everything in the store
	for _, fwObject := range firmwares.Items {
		firmwareId := fwObject.Get("id").String()
		firmwareName := fwObject.Get("name").String()
		childAdditionReferences, _, _ := c.c8yClient.Inventory.GetChildAdditions(c.ctx, firmwareId, &c8y.ManagedObjectOptions{
			PaginationOptions: *c8y.NewPaginationOptions(2000),
			Type:              "c8y_FirmwareBinary",
		})

		c.tenantStore.AddFirmware(FirmwareStoreFwEntry{
			TenantId: tenantName,
			MoId:     firmwareId,
			MoName:   firmwareName,
			MoType:   fwObject.Get("type").String(),
		})

		// collect all IDs from child additions
		var ids []string
		for _, e := range childAdditionReferences.References {
			mo := e.ManagedObject
			ids = append(ids, mo.ID)
		}
		// query object by object (TODO: improve this to not send a GET for each individual MO)
		for _, e := range ids {
			mo, _, _ := c.c8yClient.Inventory.GetManagedObject(c.ctx, e, nil)

			isPatch := mo.Item.Get("c8y_Patch").Exists()
			patchDependency := mo.Item.Get("c8y_Patch.dependency").String()
			fw := FirmwareStoreVersionEntry{
				TenantId:        tenantName,
				MoId:            mo.Item.Get("id").String(),
				Name:            firmwareName,
				Type:            mo.Item.Get("type").String(),
				IsPatch:         isPatch,
				PatchDependency: patchDependency,
				Version:         mo.Item.Get("c8y_Firmware.version").String(),
				URL:             mo.Item.Get("c8y_Firmware.url").String(),
			}
			c.tenantStore.AddFirmwareVersion(fw)
		}
	}
}

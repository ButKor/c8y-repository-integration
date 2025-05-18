package app

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

func (c *FirmwareTenantControllers) NotifyExtIndexChanged(extFwVersionEntries []ExtFirmwareVersionEntry, extFwInfoEntries map[string]ExtFirmwareInfoEntry) {
	for _, e := range c.tenantControllers {
		e.ExternalStorageIndexChanged(extFwVersionEntries, extFwInfoEntries)
	}
}

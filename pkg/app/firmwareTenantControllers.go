package app

type FirmwareTenantControllers struct {
	tenantControllers []FirmwareTenantController
}

func (c *FirmwareTenantControllers) Register(fc FirmwareTenantController) {
	if c.tenantControllers == nil {
		c.tenantControllers = make([]FirmwareTenantController, 1)
	}
	c.tenantControllers = append(c.tenantControllers, fc)
}

func (c *FirmwareTenantControllers) NotifyExtIndexChanged(indexEntries []FirmwareIndexEntry) {
	for _, e := range c.tenantControllers {
		e.ExternalStorageIndexChanged(indexEntries)
	}
}

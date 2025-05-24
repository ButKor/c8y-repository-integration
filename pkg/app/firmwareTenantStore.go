package app

type FirmwareTenantStore struct {
	// key = firmware name, value = all firmware versions
	FirmwareVersionsByName map[string][]FirmwareStoreVersionEntry
	// key=firmware name, value = firmware object
	FirmwareByName map[string]FirmwareStoreFwEntry
}

type FirmwareStoreFwEntry struct {
	TenantId string `json:"tenantId"`
	MoId     string `json:"id"`
	MoName   string `json:"name"`
	MoType   string `json:"type"`
}

type FirmwareStoreVersionEntry struct {
	TenantId        string `json:"tenantId"`
	MoId            string `json:"id"`
	MoType          string `json:"type"`
	FwName          string `json:"name"`
	IsPatch         bool   `json:"isPatch"`
	PatchDependency string `json:"patchDependency"`
	Version         string `json:"version"`
	URL             string `json:"url"`
}

func (store *FirmwareTenantStore) AddFirmware(e FirmwareStoreFwEntry) {
	store.FirmwareByName[e.MoName] = e
}

func (store *FirmwareTenantStore) AddFirmwareVersion(e FirmwareStoreVersionEntry) {
	val, ok := store.FirmwareVersionsByName[e.FwName]
	if ok {
		store.FirmwareVersionsByName[e.FwName] = append(val, e)
	} else {
		store.FirmwareVersionsByName[e.FwName] = []FirmwareStoreVersionEntry{e}
	}
}

func (store *FirmwareTenantStore) GetFirmware(fwName string) (FirmwareStoreFwEntry, bool) {
	val, ok := store.FirmwareByName[fwName]
	if ok {
		return val, ok
	}
	return FirmwareStoreFwEntry{}, false
}

func (store *FirmwareTenantStore) GetFirmwareVersion(fwName string, fwVersion string) (FirmwareStoreVersionEntry, bool) {
	val, ok := store.FirmwareVersionsByName[fwName]
	if ok {
		for _, e := range val {
			if e.Version != fwVersion {
				continue
			}
			return e, true
		}
	}
	return FirmwareStoreVersionEntry{}, false
}

func (store *FirmwareTenantStore) Flush() {
	store.FirmwareVersionsByName = make(map[string][]FirmwareStoreVersionEntry)
	store.FirmwareByName = make(map[string]FirmwareStoreFwEntry)
}

package manager

import (
	"fmt"
	"strconv"
	"time"
)

type LUNAllocateMode string

const (
	LUNAllocateMode_Thin  LUNAllocateMode = "1"
	LUNAllocateMode_Thick LUNAllocateMode = "0"
)

type getStoragePoolListResponse struct {
	DiskManageModel string `xml:"DiskManageModel"`
	AuthPassed      int    `xml:"authPassed"`
	PoolIndex       struct {
		Row []struct {
			PoolID       int `xml:"poolID"`
			PoolTiering  int `xml:"pool_tiering"`
			TierThinPool int `xml:"tier_thin_pool"`
			PoolVjbod    int `xml:"pool_vjbod"`
			AllowSysVol  int `xml:"allow_sys_vol"`
			PoolTr       int `xml:"pool_tr"`
			SedPool      int `xml:"sed_pool"`
			RemovingType int `xml:"removing_type"`
		} `xml:"row"`
	} `xml:"Pool_Index"`
	Result string `xml:"result"`
}

// GetStoragePools retrieves the list of storage pools.
func (s *QnapSession) GetStoragePools() ([]*StoragePool, error) {
	var result getStoragePoolListResponse

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("store", "poolList").
		SetQueryParam("func", "extra_get").
		SetQueryParam("extra_pool_index", "1").
		SetResult(&result).
		Post("cgi-bin/disk/disk_manage.cgi")
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return nil, fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	if result.Result != "0" {
		return nil, fmt.Errorf("failed to perform request: unexpected result code: %v", result.Result)
	}

	// read the pool info for every pool
	pools := make([]*StoragePool, 0)

	for _, poolID := range result.PoolIndex.Row {
		info, err := s.getStoragePoolInfo(poolID.PoolID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve storage pool information for pool #%v: %w", poolID.PoolID, err)
		}
		pools = append(pools, info)
	}

	return pools, nil
}

type StoragePool struct {
	PoolID                       int   `xml:"poolID"`
	PoolTiering                  int   `xml:"pool_tiering"`
	TierThinPool                 int   `xml:"tier_thin_pool"`
	PoolVjbod                    int   `xml:"pool_vjbod"`
	AllowSysVol                  int   `xml:"allow_sys_vol"`
	PoolTr                       int   `xml:"pool_tr"`
	SedPool                      int   `xml:"sed_pool"`
	RemovingType                 int   `xml:"removing_type"`
	PoolStatus                   int   `xml:"pool_status"`
	PoolType                     int   `xml:"pool_type"`
	PoolOverThreshold            int   `xml:"pool_over_threshold"`
	PoolFullType                 int   `xml:"pool_full_type"`
	CapacityBytes                int64 `xml:"capacity_bytes"`
	AllocatedBytes               int64 `xml:"allocated_bytes"`
	FreesizeBytes                int64 `xml:"freesize_bytes"`
	UnutilizedSpaceBytes         int64 `xml:"unutilized_space_bytes"`
	MaxThickCreateSizeBytes      int64 `xml:"max_thick_create_size_bytes"`
	TpReservedSizeBytes          int64 `xml:"tp_reserved_size_bytes"`
	SnapshotReservedEnable       int   `xml:"snapshot_reserved_enable"`
	SnapshotReserved             int   `xml:"snapshot_reserved"`
	SetSnapshotReservedBytes     int64 `xml:"set_snapshot_reserved_bytes"`
	RealFreesizeBytes            int64 `xml:"real_freesize_bytes"`
	SnapshotBytes                int64 `xml:"snapshot_bytes"`
	SnapshotReservedBytes        int64 `xml:"snapshot_reserved_bytes"`
	PoolAllocatedNoSnapshotBytes int64 `xml:"pool_allocated_no_snapshot_bytes"`
	VolAllocating                int   `xml:"vol_allocating"`
	TieringProcessing            int   `xml:"tiering_processing"`
	RecoverFromReadDeleteKb      int64 `xml:"recover_from_read_delete_kb"`
	OpTotalReserveSpaceKb        int64 `xml:"op_total_reserve_space_kb"`
	PoolStripe                   int   `xml:"pool_stripe"`
	IsTrRaid                     int   `xml:"is_tr_raid"`
	VolRemove                    int   `xml:"vol_remove"`
}

type getStoragePoolInfoResponse struct {
	AuthPassed      int    `xml:"authPassed"`
	DiskManageModel string `xml:"DiskManageModel"`
	PoolIndex       struct {
		SingleRow *StoragePool `xml:"row"`
	} `xml:"Pool_Index"`

	Result string `xml:"result"`
}

func (s *QnapSession) getStoragePoolInfo(poolID int) (*StoragePool, error) {
	var result getStoragePoolInfoResponse

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("store", "poolInfo").
		SetQueryParam("func", "extra_get").
		SetQueryParam("Pool_Info", "1").
		SetQueryParam("poolID", strconv.Itoa(poolID)).
		SetResult(&result).
		Post("cgi-bin/disk/disk_manage.cgi")
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return nil, fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	if result.Result != "0" {
		return nil, fmt.Errorf("failed to perform request: unexpected result code: %v", result.Result)
	}
	if result.PoolIndex.SingleRow == nil {
		return nil, fmt.Errorf("response does not contain any pool information")
	}

	return result.PoolIndex.SingleRow, nil
}

type createBlockBasedLUNResponse struct {
	AuthPassed int    `xml:"authPassed"`
	ISCSIModel string `xml:"iSCSIModel"`
	VolumeID   int    `xml:"volumeID"`
	LUNIndex   int    `xml:"result"`
}

// CreateBlockBasedLUN creates a new block-based volume inside a storage pool and returns the new LUN.
func (s *QnapSession) CreateBlockBasedLUN(storagePoolID int, name string, capacityGB int, allocateMode LUNAllocateMode, useSSDCache bool, alertThresoldPercent int) (*LUN, error) {
	var result createBlockBasedLUNResponse

	useSSDCacheStr := "no"
	if useSSDCache {
		useSSDCacheStr = "yes"
	}

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("func", "add_lun").
		SetQueryParam("LUNThinAllocate", string(allocateMode)).
		SetQueryParam("LUNName", name).
		SetQueryParam("LUNCapacity", strconv.Itoa(capacityGB)).
		SetQueryParam("LUNSectorSize", "512"). // default for linux
		SetQueryParam("WCEnable", "0").
		SetQueryParam("FUAEnable", "0").
		SetQueryParam("FileIO", "0").
		SetQueryParam("poolID", strconv.Itoa(storagePoolID)).
		SetQueryParam("lv_ifssd", useSSDCacheStr).
		SetQueryParam("LUNPath", name).
		SetQueryParam("enable_tiering", "0").
		SetQueryParam("lv_threshold", strconv.Itoa(alertThresoldPercent)).
		SetResult(&result).
		Post("cgi-bin/disk/iscsi_lun_setting.cgi")
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return nil, fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}

	// find the lun (need to try several times)
	for try := 1; try <= 30; try++ {
		time.Sleep(2 * time.Second) // wait two seconds

		lun, err := s.GetLUNByIndex(result.LUNIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to get LUN %v: %w", result.LUNIndex, err)
		}
		if lun != nil {
			return lun, nil
		}
	}

	return nil, fmt.Errorf("failed to find LUN %v (timeout)", result.LUNIndex)
}

type LUN struct {
	LUNIndex            int    `xml:"LUNIndex"`
	LUNName             string `xml:"LUNName"`
	LUNPath             string `xml:"LUNPath"`
	LUNStatus           int    `xml:"LUNStatus"`
	LUNThinAllocate     bool   `xml:"LUNThinAllocate"`
	LUNAttachedTarget   int    `xml:"LUNAttachedTarget"`
	LUNNumber           int    `xml:"LUNNumber"`
	LUNSerialNum        string `xml:"LUNSerialNum"`
	LUNBackupStatus     int    `xml:"LUNBackupStatus"`
	IsSnap              int    `xml:"isSnap"`
	IsRemoving          int    `xml:"isRemoving"`
	BMap                int    `xml:"bMap"`
	CapacityBytes       int64  `xml:"capacity_bytes"`
	VolumeBase          string `xml:"VolumeBase"`
	WCEnable            bool   `xml:"WCEnable"`
	FUAEnable           bool   `xml:"FUAEnable"`
	LUNThresholdPercent int    `xml:"LUNThreshold"`
	LUNNAA              string `xml:"LUNNAA"`
	LUNSectorSize       int    `xml:"LUNSectorSize"`
	SsdCache            string `xml:"ssd_cache"`
	PoolID              int    `xml:"poolID"`
	PoolVjbod           bool   `xml:"pool_vjbod"`
	VolumeID            int    `xml:"volno"`
	BlockSize           int64  `xml:"block_size"`
	LUNTargetList       struct {
		SingleRow *struct {
			TargetIndex int  `xml:"targetIndex"`
			LUNNumber   int  `xml:"LUNNumber"`
			LUNEnable   bool `xml:"LUNEnable"`
		} `xml:"row"`
	} `xml:"LUNTargetList"`
	LUNInitiatorList struct {
		LUNInitInfo []struct {
			InitiatorIndex int    `xml:"initiatorIndex"`
			InitiatorIQN   string `xml:"initiatorIQN"`
			AccessMode     int    `xml:"accessMode"`
		} `xml:"LUNInitInfo"`
	} `xml:"LUNInitList"`
}

type getStorageLUNsResponse struct {
	AuthPassed   int    `xml:"authPassed"`
	ISCSIModel   string `xml:"iSCSIModel"`
	ISCSILUNList struct {
		LUNInfo []*LUN `xml:"LUNInfo"`
	} `xml:"iSCSILUNList"`
	Result string `xml:"result"`
}

// GetLUNs retrieves the list of all storage LUNs.
func (s *QnapSession) GetLUNs() ([]*LUN, error) {
	var result getStorageLUNsResponse

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("store", "storageSpace_LUNList").
		SetQueryParam("func", "extra_get").
		SetQueryParam("lunList", "1").
		SetResult(&result).
		Post("cgi-bin/disk/iscsi_portal_setting.cgi")
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return nil, fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	if result.Result != "0" {
		return nil, fmt.Errorf("failed to perform request: unexpected result code: %v", result.Result)
	}

	return result.ISCSILUNList.LUNInfo, nil
}

type getLUNByID struct {
	AuthPassed int    `xml:"authPassed"`
	ISCSIModel string `xml:"iSCSIModel"`
	LUNInfo    struct {
		SingleRow *LUN `xml:"row"`
	} `xml:"LUNInfo"`
	Result string `xml:"result"`
}

// GetLUNByIndex retrieves the a storage LUN by its LUN ID (not volume ID!)
func (s *QnapSession) GetLUNByIndex(lunIndex int) (*LUN, error) {
	var result getLUNByID

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("store", "lunInfo").
		SetQueryParam("lunID", strconv.Itoa(lunIndex)).
		SetQueryParam("func", "extra_get").
		SetQueryParam("lun_info", "1").
		SetResult(&result).
		Post("cgi-bin/disk/iscsi_portal_setting.cgi")
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return nil, fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	if result.Result != "0" {
		return nil, fmt.Errorf("failed to perform request: unexpected result code: %v", result.Result)
	}

	return result.LUNInfo.SingleRow, nil
}

type genericResponse struct {
	AuthPassed int    `xml:"authPassed"`
	ISCSIModel string `xml:"iSCSIModel"`
	Result     string `xml:"result"`
}

// DeleteLUN retrieves the a storage LUN by its LUN ID (not volume ID!)
func (s *QnapSession) DeleteLUN(lunID int) error {
	var result genericResponse

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("prod", "qts").
		SetQueryParam("proto", "iscsi").
		SetQueryParam("target", "lio").
		SetQueryParam("backend", "dm").
		SetQueryParam("conf", "init").
		SetQueryParam("func", "remove_lun").
		SetQueryParam("run_background", "1").
		SetQueryParam("LUNIndex", strconv.Itoa(lunID)).
		SetResult(&result).
		Post("cgi-bin/disk/iscsi_lun_setting.cgi")
	if err != nil {
		return fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	if result.Result != "0" {
		return fmt.Errorf("failed to perform request: unexpected result code: %v", result.Result)
	}

	return nil
}

// WaitForLUNVolume waits for the volume of the LUN to become ready.
func (s *QnapSession) WaitForLUNVolume(lunID int) (*LUN, error) {
	for try := 1; try <= 30; try++ {
		lun, err := s.GetLUNByIndex(lunID)
		if err != nil {
			return nil, fmt.Errorf("failed to get LUN %v: %w", lunID, err)
		}
		if lun == nil {
			return nil, fmt.Errorf("LUN not found: %v", lunID)
		}
		if lun.VolumeID >= 0 { // volume is ready
			return lun, nil
		}

		time.Sleep(2 * time.Second) // wait two seconds
	}

	return nil, fmt.Errorf("failed to wait for LUN %v volume to become ready (timeout)", lunID)
}

// AssignLUN assigns an existing LUN to an existing iSCSI target
func (s *QnapSession) AssignLUN(lunIndex int, targetIndex int) error {
	var result genericResponse

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("prod", "qts").
		SetQueryParam("proto", "iscsi").
		SetQueryParam("target", "lio").
		SetQueryParam("backend", "dm").
		SetQueryParam("conf", "ini").
		SetQueryParam("func", "add_lun").
		SetQueryParam("LUNIndex", strconv.Itoa(lunIndex)).
		SetQueryParam("targetIndex", strconv.Itoa(targetIndex)).
		SetResult(&result).
		Post("cgi-bin/disk/iscsi_target_setting.cgi")
	if err != nil {
		return fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	// do not check for result.Result as it contains the LUN LUNIndex within the iSCSI target

	return nil
}

type ISCSITarget struct {
	TargetIndex  int    `xml:"targetIndex"`
	TargetName   string `xml:"targetName"`
	TargetIQN    string `xml:"targetIQN"`
	TargetAlias  string `xml:"targetAlias"`
	TargetStatus int    `xml:"targetStatus"`
}

type getISCSITargetsResponse struct {
	AuthPassed      int    `xml:"authPassed"`
	ISCSIModel      string `xml:"iSCSIModel"`
	ISCSITargetList struct {
		TargetInfo []*ISCSITarget `xml:"targetInfo"`
	} `xml:"iSCSITargetList"`
	Result string `xml:"result"`
}

// GetISCSITargets retrieves the list of all iSCSI targets.
func (s *QnapSession) GetISCSITargets() ([]*ISCSITarget, error) {
	var result getISCSITargetsResponse

	res, err := s.conn.NewRequest().
		ExpectContentType("text/xml").
		SetQueryParam("prod", "qts").
		SetQueryParam("proto", "iscsi").
		SetQueryParam("target", "lio").
		SetQueryParam("backend", "dm").
		SetQueryParam("conf", "ini").
		SetQueryParam("func", "extra_get").
		SetQueryParam("targetList", "1").
		SetResult(&result).
		Post("cgi-bin/disk/iscsi_portal_setting.cgi")
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return nil, fmt.Errorf("failed to perform request: authentication invalid: %v", string(res.Body()))
	}
	if result.Result != "0" {
		return nil, fmt.Errorf("failed to perform request: unexpected result code: %v", result.Result)
	}

	return result.ISCSITargetList.TargetInfo, nil
}

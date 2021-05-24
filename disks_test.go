package manager

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestDisksRoundtrip(t *testing.T) {
	s := createTestSession(t)
	defer s.Logout()

	rand.Seed(time.Now().UTC().UnixNano())

	pools, err := s.GetStoragePools()
	if err != nil {
		t.Fatalf("Failed retrieve storage pool list: %v", err)
	}
	if len(pools) <= 0 {
		t.Fatalf("No storage pool found")
	}

	targets, err := s.GetISCSITargets()
	if err != nil {
		t.Fatalf("Failed retrieve iSCSI target list: %v", err)
	}
	if len(targets) <= 0 {
		t.Fatalf("No iSCSI target found")
	}

	target := targets[0]

	t.Logf("Using iSCSI Target: %v (#%v)", target.TargetIQN, target.TargetIndex)

	for _, pool := range pools {
		lunName := fmt.Sprintf("UnitTest_%v", 10000+rand.Int31n(89999))

		t.Logf("Using LUN/volume name: %v", lunName)

		// create the lun
		var lun *LUN

		t.Run(fmt.Sprintf("Test_Storage Pool %v_CreateLUN", pool.PoolID), func(t *testing.T) {
			lun, err = s.CreateBlockBasedLUN(pool.PoolID, lunName, 1, LUNAllocateMode_Thin, true, 99)
			if err != nil {
				t.Fatalf("Failed to create LUN '%v': %v", lunName, err)
			}

			t.Logf("Created new LUN %v", lun.LUNIndex)

			if lun.VolumeID != -1 {
				t.Fatalf("Unexpected volume ID (should not exist, yet)")
			}
			if lun.LUNTargetList.SingleRow != nil {
				t.Fatalf("Unexpected iSCSI target information (not assigned, yet)")
			}
		})

		// wait for the volume
		t.Run(fmt.Sprintf("Test_Storage Pool %v_WaitForVolume", pool.PoolID), func(t *testing.T) {
			lun, err = s.WaitForLUNVolume(lun.LUNIndex)
			if err != nil {
				t.Fatalf("Failed to wait for volume of LUN '%v': %v", lunName, err)
			}
		})

		// assign the LUN
		t.Run(fmt.Sprintf("Test_Storage Pool %v_AssignToTarget", pool.PoolID), func(t *testing.T) {
			err = s.AssignLUN(lun.LUNIndex, target.TargetIndex)
			if err != nil {
				t.Fatalf("Failed to assign LUN '%v' to iSCSI target '%v': %v", lunName, "xxxx", err)
			}
		})

		// check the target
		t.Run(fmt.Sprintf("Test_Storage Pool %v_CheckTarget", pool.PoolID), func(t *testing.T) {
			lun, err = s.GetLUNByIndex(lun.LUNIndex)
			if err != nil {
				t.Fatalf("Failed to get LUN '%v' to iSCSI target '%v': %v", lunName, "xxxx", err)
			}
			if lun.LUNTargetList.SingleRow == nil {
				t.Fatalf("Missing iSCSI target information")
			}
		})

		// delete the lun
		t.Run(fmt.Sprintf("Test_Storage Pool %v_DeleteLUN", pool.PoolID), func(t *testing.T) {
			err := s.DeleteLUN(lun.LUNIndex)
			if err != nil {
				t.Fatalf("Failed to delete LUN '%v': %v", lunName, err)
			}
		})
	}
}

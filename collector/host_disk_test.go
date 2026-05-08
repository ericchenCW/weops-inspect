package collector

import (
	"strings"
	"testing"
)

// build60DfThP is a real `df -ThP` output captured from a BUILD60 host
// (LVM root at 98%, NFS-mounted share, dozens of overlay/tmpfs entries).
const build60DfThP = `Filesystem                             Type      Size  Used Avail Use% Mounted on
devtmpfs                               devtmpfs   16G     0   16G   0% /dev
tmpfs                                  tmpfs      16G  2.3M   16G   1% /dev/shm
tmpfs                                  tmpfs      16G  1.7G   14G  11% /run
tmpfs                                  tmpfs      16G     0   16G   0% /sys/fs/cgroup
/dev/mapper/klas-root                  xfs       291G  283G  8.3G  98% /
tmpfs                                  tmpfs      16G   20M   16G   1% /tmp
/dev/vda2                              xfs      1014M  216M  799M  22% /boot
/dev/vda1                              vfat      599M  6.5M  593M   2% /boot/efi
10.11.24.63:/data/bkee/public/nfs/saas nfs4      141G   84G   57G  60% /data/bkee/public/paas_agent/share
overlay                                overlay   291G  283G  8.3G  98% /data/bkee/public/paas_agent/docker/overlay2/abc/merged
shm                                    tmpfs      64M     0   64M   0% /data/bkee/public/paas_agent/docker/containers/xyz/mounts/shm
tmpfs                                  tmpfs     3.1G     0  3.1G   0% /run/user/0
`

// build60DfIPT is the inode equivalent (df -iPT). Same row shape, different
// numeric columns; the parser is shared.
const build60DfIPT = `Filesystem                             Type      Inodes  IUsed   IFree IUse% Mounted on
/dev/mapper/klas-root                  xfs    152428096 950000 151478096   1% /
tmpfs                                  tmpfs    4096000     20  4095980    1% /tmp
/dev/vda2                              xfs       524288    420   523868    1% /boot
overlay                                overlay 152428096 950000 151478096   1% /data/bkee/public/paas_agent/docker/overlay2/abc/merged
`

// longLVMDf exercises POSIX single-line output with a very long device name
// that would have wrapped under non-POSIX `df -h`.
const longLVMDf = `Filesystem                                                                Type  Size  Used Avail Use% Mounted on
/dev/mapper/very-long-volume-group-name-very-long-logical-volume-name-root xfs   500G  300G  200G  60% /
`

// xfsAndExt4Mix gives a mixed-fs host without any noise filesystems.
const xfsAndExt4Mix = `Filesystem  Type  Size  Used Avail Use% Mounted on
/dev/sda1   xfs   100G   50G   50G  50% /
/dev/sda2   ext4   50G   20G   30G  40% /var
/dev/sdb1   ext4  500G  100G  400G  20% /home
`

func mountPointsOf(dus []struct{ mp, fs string }) []string {
	var mps []string
	for _, du := range dus {
		mps = append(mps, du.mp)
	}
	return mps
}

func collected(out []byte) {} // silence unused-imports linter if any

func Test_parseDiskUsage_DefaultFiltersBuild60(t *testing.T) {
	// Default path: CHECK_MOUNT_PATH empty, NFS off.
	results, _ := parseDiskUsage(build60DfThP, "", false)

	gotMounts := map[string]string{}
	for _, du := range results {
		gotMounts[du.MountPoint] = du.FsType
	}

	// /, /boot (xfs) and /boot/efi (vfat) are real persistent filesystems
	// and must all survive the default filter.
	want := map[string]string{
		"/":         "xfs",
		"/boot":     "xfs",
		"/boot/efi": "vfat",
	}
	for mp, fs := range want {
		if got, ok := gotMounts[mp]; !ok || got != fs {
			t.Errorf("expected %s=%s in results, got %v (full=%v)", mp, fs, got, gotMounts)
		}
	}

	// All overlay / tmpfs / devtmpfs / shm / nfs entries must be excluded.
	for mp, fs := range gotMounts {
		if fs == "overlay" || fs == "tmpfs" || fs == "devtmpfs" || fs == "nfs4" || fs == "shm" {
			t.Errorf("unexpected noisy fs %s at %s", fs, mp)
		}
	}

	// The 98% root must show up with parsed UsageFloat.
	var rootDu *struct {
		Usage      string
		UsageFloat float64
	}
	_ = rootDu
	for _, du := range results {
		if du.MountPoint == "/" {
			if du.UsageFloat != 98 {
				t.Errorf("expected / UsageFloat=98, got %v", du.UsageFloat)
			}
			if du.Usage != "98%" {
				t.Errorf("expected / Usage=98%%, got %q", du.Usage)
			}
		}
	}
}

func Test_parseDiskUsage_ExplicitDataNotMatchedOnBuild60(t *testing.T) {
	// User configured /data, but BUILD60 has no /data mount itself.
	// Expect empty result + caller can produce a warning.
	results, seen := parseDiskUsage(build60DfThP, "/data", false)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d (%v)", len(results), results)
	}
	if len(seen) == 0 {
		t.Fatal("expected seenMounts to be populated for warning context")
	}

	// Verify the warning helper produces a useful message.
	warn := appendDiskWarning("", "/data", seen)
	if !strings.Contains(warn, "/data") {
		t.Errorf("warning should mention configured /data, got %q", warn)
	}
	if !strings.Contains(warn, "did not match") {
		t.Errorf("warning should say 'did not match', got %q", warn)
	}
}

func Test_parseDiskUsage_NFSGate(t *testing.T) {
	// Default: NFS excluded.
	results, _ := parseDiskUsage(build60DfThP, "", false)
	for _, du := range results {
		if du.FsType == "nfs4" {
			t.Errorf("NFS should be excluded by default, got %v", du)
		}
	}

	// With include flag: NFS included.
	results, _ = parseDiskUsage(build60DfThP, "", true)
	var foundNFS bool
	for _, du := range results {
		if du.FsType == "nfs4" {
			foundNFS = true
			if du.MountPoint != "/data/bkee/public/paas_agent/share" {
				t.Errorf("unexpected NFS mountpoint %q", du.MountPoint)
			}
		}
	}
	if !foundNFS {
		t.Error("expected NFS mount to be collected when INSPECT_DISK_INCLUDE_NFS=true")
	}
}

func Test_parseDiskUsage_LongLVMDeviceName(t *testing.T) {
	// `df -ThP` keeps everything on one line regardless of device-name length.
	results, _ := parseDiskUsage(longLVMDf, "", false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for long-LVM fixture, got %d", len(results))
	}
	if results[0].MountPoint != "/" || results[0].FsType != "xfs" {
		t.Errorf("expected /,xfs got %q,%q", results[0].MountPoint, results[0].FsType)
	}
	if results[0].UsageFloat != 60 {
		t.Errorf("expected UsageFloat=60, got %v", results[0].UsageFloat)
	}
}

func Test_parseDiskUsage_InodeReusesParser(t *testing.T) {
	// Same parser handles `df -iPT`. Verify FsType is filled and overlay is
	// still filtered.
	results, _ := parseDiskUsage(build60DfIPT, "", false)
	if len(results) == 0 {
		t.Fatal("expected at least one inode result")
	}
	for _, du := range results {
		if du.FsType == "" {
			t.Errorf("FsType missing for %s", du.MountPoint)
		}
		if du.FsType == "overlay" || du.FsType == "tmpfs" {
			t.Errorf("overlay/tmpfs should be excluded from inode output too, got %v", du)
		}
	}

	// / and /boot (xfs) should both be present.
	mounts := map[string]bool{}
	for _, du := range results {
		mounts[du.MountPoint] = true
	}
	if !mounts["/"] || !mounts["/boot"] {
		t.Errorf("expected / and /boot in inode results, got %v", mounts)
	}
}

func Test_parseDiskUsage_BlocklistOverridesExplicitConfig(t *testing.T) {
	// Even if the user explicitly configures /run (tmpfs), it must be excluded
	// because tmpfs is on the always-block list.
	df := `Filesystem  Type   Size  Used Avail Use% Mounted on
tmpfs       tmpfs   16G  1.7G   14G  11% /run
/dev/sda1   xfs    100G   50G   50G  50% /
`
	results, _ := parseDiskUsage(df, "/run:/", false)
	if len(results) != 1 || results[0].MountPoint != "/" {
		t.Errorf("expected only / to survive blocklist filter, got %v", results)
	}
}

func Test_parseDiskUsage_ExplicitListExactMatchOnly(t *testing.T) {
	// Configuring /data must NOT prefix-match /data/share.
	df := `Filesystem  Type  Size  Used Avail Use% Mounted on
/dev/sda1   xfs   100G   50G   50G  50% /
/dev/sdb1   xfs   500G  100G  400G  20% /data/share
`
	results, _ := parseDiskUsage(df, "/data", false)
	if len(results) != 0 {
		t.Errorf("expected 0 results (no exact /data match), got %v", results)
	}
}

func Test_shouldCollectFs(t *testing.T) {
	cases := []struct {
		fs        string
		includeNFS bool
		want      bool
	}{
		{"xfs", false, true},
		{"ext4", false, true},
		{"tmpfs", false, false},
		{"tmpfs", true, false}, // blocklist always wins
		{"overlay", true, false},
		{"nfs4", false, false},
		{"nfs4", true, true},
		{"vfat", false, true}, // boot/efi partition – monitored
		{"unknownfs", false, false},
	}
	for _, c := range cases {
		if got := shouldCollectFs(c.fs, c.includeNFS); got != c.want {
			t.Errorf("shouldCollectFs(%q, %v) = %v, want %v", c.fs, c.includeNFS, got, c.want)
		}
	}
}

func Test_parseDiskUsage_DefaultEmptyResultProducesWarning(t *testing.T) {
	// Host where every mount is excluded by the default filter.
	df := `Filesystem  Type    Size  Used Avail Use% Mounted on
tmpfs       tmpfs    16G     0   16G   0% /dev/shm
overlay     overlay 100G   80G   20G  80% /var/lib/docker/overlay2/x/merged
`
	results, seen := parseDiskUsage(df, "", false)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
	warn := appendDiskWarning("", "", seen)
	if !strings.Contains(warn, "no real filesystem") {
		t.Errorf("expected default-mode warning, got %q", warn)
	}
}

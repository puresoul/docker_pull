package vmbetter

type Conf struct {
F_debian_mirror string
F_noclean bool
F_stage1 bool
F_stage2 string
F_branch string
F_disk bool
F_diskSize string
F_format string
F_mbr string
F_iso bool
F_isolinux string
F_rootfs bool
F_dstrp_append string
F_constraints string
F_target string
F_dry_run bool
}

var (
    CF *Conf
)

func init() {
var cf Conf
cf.F_debian_mirror = "http://mirrors.ocf.berkeley.edu/debian"
cf.F_noclean = false
cf.F_stage1 = false
cf.F_stage2 = ""
cf.F_branch = "testing"
cf.F_disk = false
cf.F_diskSize = "1G"
cf.F_format = "qcow2"
cf.F_mbr = "/usr/lib/syslinux/mbr/mbr.bin"
cf.F_iso = false
cf.F_isolinux = "misc/isolinux/"
cf.F_rootfs = false
cf.F_dstrp_append = ""
cf.F_constraints = "debianamd64"
cf.F_target = ""
cf.F_dry_run = false
CF = &cf
}
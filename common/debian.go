package common

import (
    "os"
)

func DpkgPackageExists(pkg string) bool {
    _, err := os.Stat("/var/lib/dpkg/info/" + pkg + ".list")
    return err == nil
}

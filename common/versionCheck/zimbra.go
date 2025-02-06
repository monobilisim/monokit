//go:build linux
package common

import (
    "os"
    "fmt"
    "strings"
    "os/exec"
)

func ZimbraCheck() {
    var zimbraPath string
    var zimbraUser string

    if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
        zimbraPath = "/opt/zimbra"
        zimbraUser = "zimbra"
    }

    if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
        zimbraPath = "/opt/zextras"
        zimbraUser = "zextras"
    }

    // Get the version of Zimbra
    cmd := exec.Command("/bin/su", zimbraUser, "-c", zimbraPath + "/bin/zmcontrol -v")
    out, err := cmd.Output()
    if err != nil {
        fmt.Println("Error getting Zimbra version.")
        return
    }

    // Parse the version
    // Eg. output
    // Release 8.8.15_GA_3869.UBUNTU18.64 UBUNTU18_64 FOSS edition.
    version := strings.Split(string(out), " ")[1]
    version = strings.Split(version, "_GA_")[0]

    fmt.Println("Zimbra version:", version)

    oldVersion := GatherVersion("zimbra")

    if oldVersion != "" && oldVersion == version {
        fmt.Println("Zimbra is not updated yet.")
        return
    } else if oldVersion != "" && oldVersion != version {
        fmt.Println("Zimbra has been updated.")
        fmt.Println("Old version:", oldVersion)
        fmt.Println("New version:", version)
        CreateNews("Zimbra", oldVersion, version)
    }

    StoreVersion("zimbra", version)
}

package common

import (
    "fmt"
    "os/exec"
    "strings"
)

func OPNsenseCheck() {
    // Check if OPNsense is installed by checking the existence of command "opnsense-version"
    _, err := exec.LookPath("opnsense-version")
    if err != nil {
        fmt.Println("OPNsense is not installed on this system.")
        return
    }

    // Get the version of OPNsense
    out, err := exec.Command("opnsense-version").Output()
    if err != nil {
        fmt.Println("Error getting OPNsense version.")
        return
    }

    // Parse the version
    // Eg. output
    // OPNsense 21.1.8_1 (amd64)
    version := strings.Split(string(out), " ")[1]
    fmt.Println("OPNsense version:", version)
    
    oldVersion := GatherVersion("opnsense")

    if oldVersion != "" && oldVersion == version {
        fmt.Println("OPNsense is up to date.")
        return
    } else if oldVersion != "" && oldVersion != version {
        fmt.Println("OPNsense got updated.")
        fmt.Println("Old version:", oldVersion)
        fmt.Println("New version:", version)
        CreateNews("OPNsense", oldVersion, version)
    }


    StoreVersion("opnsense", version)
}

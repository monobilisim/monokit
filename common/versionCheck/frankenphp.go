package common

import (
    "fmt"
    "os/exec"
    "strings"
)

// FrankenPHPCheck detects the installed FrankenPHP version and reports updates
func FrankenPHPCheck() {
    // Check if FrankenPHP binary exists
    _, err := exec.LookPath("frankenphp")
    if err != nil {
        fmt.Println("FrankenPHP is not installed on this system.")
        return
    }

    // Get the version output
    out, err := exec.Command("frankenphp", "-v").Output()
    if err != nil {
        fmt.Println("Error getting FrankenPHP version.")
        return
    }

    // Expected example output:
    // FrankenPHP v1.9.0 PHP 8.4.10 Caddy v2.10.0 h1:...
    fields := strings.Fields(string(out))
    if len(fields) < 2 {
        fmt.Println("Unexpected FrankenPHP version output.")
        return
    }
    // fields[1] should be like "v1.9.0"
    version := strings.TrimPrefix(fields[1], "v")

    fmt.Println("FrankenPHP version:", version)

    oldVersion := GatherVersion("frankenphp")

    if oldVersion != "" && oldVersion == version {
        fmt.Println("FrankenPHP has not been updated.")
        return
    } else if oldVersion != "" && oldVersion != version {
        fmt.Println("FrankenPHP got updated.")
        fmt.Println("Old version:", oldVersion)
        fmt.Println("New version:", version)
        CreateNews("FrankenPHP", oldVersion, version, false)
    }

    StoreVersion("frankenphp", version)
}

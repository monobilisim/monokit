package common

import (
    "fmt"
    "os/exec"
    "strings"
)

func PostgresCheck() {
    // Check if PostgreSQL is installed by checking the existence of command "psql"
    _, err := exec.LookPath("psql")
    if err != nil {
        fmt.Println("PostgreSQL is not installed on this system.")
        return
    }

    // Get the version of PostgreSQL
    out, err := exec.Command("psql", "--version").Output()
    if err != nil {
        fmt.Println("Error getting PostgreSQL version.")
        return
    }

    // Parse the version
    // Eg. output
    // psql (PostgreSQL) 13.3 (Ubuntu 13.3-1.pgdg20.04+1)
    version := strings.Split(string(out), " ")[2]

    fmt.Println("PostgreSQL version:", version)

    oldVersion := GatherVersion("postgres")

    if oldVersion != "" && oldVersion == version {
        fmt.Println("PostgreSQL has not been updated.")
        return
    }

    if oldVersion != "" && oldVersion != version {
        fmt.Println("PostgreSQL got updated.")
        fmt.Println("Old version:", oldVersion)
        fmt.Println("New version:", version)
        CreateNews("PostgreSQL", oldVersion, version)
    }

    StoreVersion("postgres", version)
}


package common

import (
    "fmt"
    "strings"
    "github.com/spf13/cobra"
)

var MigrateCmd = &cobra.Command{
    Use: "migrate",
    Short: "Migrate Monokit from one version to another",
    Run: func(cmd *cobra.Command, args []string) {
        fromVersion, _ := cmd.Flags().GetString("from")
        Migrate(fromVersion)
    },
}

func MigrateV5ToV6() {
    return // Placeholder
}

func Migrate(fromVersion string) {
    fromVersionSplit := strings.Split(fromVersion, ".")[0] // Should be v1, v2, etc.

    if fromVersionSplit == "v5" {
        fmt.Println("Migrating from v5 to v6")
        MigrateV5ToV6()
    } else {
        fmt.Println("No migration path exists for this version")
    }
}

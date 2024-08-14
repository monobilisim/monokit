package main

import (
    "github.com/monobilisim/mono-go/osHealth"
    "github.com/spf13/cobra"
    "fmt"
    "os"
)

func main() {

    var RootCmd = &cobra.Command{Use: "mono-go"}

    var osHealthCmd = &cobra.Command{
        Use:   "osHealth",
        Short: "OS Health",
        Run: osHealth.Main,
    }

    RootCmd.AddCommand(osHealthCmd)
    
    if err := RootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}

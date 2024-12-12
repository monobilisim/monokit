package common

import (
    "os"
    "fmt"
    "strings"
    "os/exec"
    "runtime"
    "net/http"
    "archive/tar"
    "encoding/json"
    "compress/gzip"
    "github.com/spf13/cobra"
)

var UpdateCmd = &cobra.Command{
    Use: "update",
    Short: "Update Monokit",
    Run: func(cmd *cobra.Command, args []string) {
        specificVersion, _ := cmd.Flags().GetString("version")
        force, _ := cmd.Flags().GetBool("force")

        Update(specificVersion, force)
    },
}

func DownloadAndExtract(url string, runMigrations bool, oldVersion string) {
    MonokitPath, err := os.Executable()

    if err != nil {
        LogError("Couldn't get executable path: " + err.Error())
    }

    // Download the release
    resp, err := http.Get(url)
    if err != nil {
        LogError("Couldn't download the release: " + err.Error())
    }
    defer resp.Body.Close()

    // Extract the release
    gzr, err := gzip.NewReader(resp.Body)

    if err != nil {
        LogError("Couldn't extract the release: " + err.Error())
    }

    defer gzr.Close()

    tr := tar.NewReader(gzr)
    for {
        hdr, err := tr.Next()
        if err != nil {
            break
        }

        if hdr.Name == "monokit" {
            f, err := os.Create(TmpDir + "monokit")
            if err != nil {
                LogError("Couldn't create monokit binary: " + err.Error())
            }
            defer f.Close()

            _, err = f.ReadFrom(tr)
            if err != nil {
                LogError("Couldn't write monokit binary: " + err.Error())
            }
        }
    }

    os.Rename(MonokitPath, MonokitPath + ".bak")

    // Move monokit binary to the correct path
    os.Rename(TmpDir + "monokit", MonokitPath)
    os.Chmod(MonokitPath, 0755)

    if runMigrations {
        cmd, err := exec.Command(MonokitPath, "migrate", "--from", oldVersion).Output()
        if err != nil {
            fmt.Println("Migration failed:", err)
            fmt.Println("Returning to the previous version")
            os.Rename(MonokitPath + ".bak", MonokitPath)
        } else {
            fmt.Println(string(cmd))
        }
    }
}

func Update(specificVersion string, force bool) {
    var url string
    var version string
    os := runtime.GOOS
    arch := runtime.GOARCH
    runMigrations := false

    if specificVersion != "" {
        version = specificVersion
        url = "https://github.com/monobilisim/monokit/releases/download/v" + specificVersion + "/monokit_" + specificVersion + "_" + os + "_" + arch + ".tar.gz"
    } else {
        // Get latest release
        url = "https://api.github.com/repos/monobilisim/monokit/releases/latest"
        resp, err := http.Get(url)
        if err != nil {
            LogError("Couldn't get latest release: " + err.Error())
        }
        defer resp.Body.Close()
        
        var release map[string]interface{}
        err = json.NewDecoder(resp.Body).Decode(&release)
        if err != nil {
            LogError("Couldn't decode latest release: " + err.Error())
        }

        assets := release["assets"].([]interface{})
        for _, asset := range assets {
            assetMap := asset.(map[string]interface{})
            if strings.Contains(assetMap["name"].(string), os) && strings.Contains(assetMap["name"].(string), arch) {
                url = assetMap["browser_download_url"].(string)
                version = release["tag_name"].(string)
                version = strings.TrimPrefix(version, "v")
                break
            }
        }
    }

    if url == "" {
        fmt.Println("No release found for your OS and architecture")
        return
    }

    if (version == MonokitVersion || MonokitVersion == "devel") && !force {
        
        if MonokitVersion == "devel" {
            fmt.Println("Monokit is a development version, run with --force to update anyway")
        } else {
            fmt.Println("Monokit is already up to date, run with --force to update anyway")
        }

        return
    }
    
    fmt.Println("Current Monokit version:", MonokitVersion)

    if MonokitVersion != "devel" {
        monokitVersionSplit := strings.Split(MonokitVersion, ".")
        versionSplit := strings.Split(version, ".")
        if monokitVersionSplit[0] != versionSplit[0] {
            if !force {
                fmt.Println("A new major version is available. This might bring breaking changes. Monokit will attempt to migrate to the new vesrion. You can run with --force to update")
                return
            } else {
                runMigrations = true
            }
        }
    }

    fmt.Println("Downloading Monokit version", version)
    DownloadAndExtract(url, runMigrations, MonokitVersion)

    fmt.Println("Monokit updated to version", version)
}

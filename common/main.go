package common

import ( 
    "os"
    "fmt"
)

var Config Common
var TmpPath string

func SplitSection(section string) {
    fmt.Println("\n" + section)
    fmt.Println("--------------------------------------------------")
}

func ConvertBytes(bytes uint64) string {
    var sizes = []string{"B", "KB", "MB", "GB", "TB", "EB"}
    var i int

    for i = 0; bytes >= 1024 && i < len(sizes); i++ {
        bytes /= 1024
    }

    return fmt.Sprintf("%d %s", bytes, sizes[i])
}

func Init() {
    // Create TmpDir if it doesn't exist
    if _, err := os.Stat(TmpDir); os.IsNotExist(err) {
        err = os.MkdirAll(TmpDir, 0755)
        
        if err != nil {
            fmt.Println("Error creating tmp directory: \n" + TmpDir + "\n" + err.Error())
            os.Exit(1)
        }

    }
    
    LogInit()
    ConfInit("global", &Config)
}

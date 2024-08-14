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

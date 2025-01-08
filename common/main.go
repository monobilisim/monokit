package common

import ( 
    "os"
    "fmt"
    "bufio"
    "unicode"
)

var Config Common
var TmpPath string
var MonokitVersion = "devel"

func SplitSection(section string) {
    fmt.Println("\n" + section)
    fmt.Println("--------------------------------------------------")
}

func ContainsUint32(a uint32, b []uint32) bool {
	for _, c := range b {
		if a == c {
			return true
		}
	}
	return false
}

func IsEmptyOrWhitespace(filePath string) bool {
        file, err := os.Open(filePath)
        if err != nil {
                fmt.Println("Error opening file:", err)
                return false // Error opening file, consider it not empty
        }
        defer file.Close()

        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
                text := scanner.Text()
                if len(text) > 0 && !isWhitespace(text) {
                        return false // Non-whitespace content found
                }
        }

        if err := scanner.Err(); err != nil {
                fmt.Println("Error reading file:", err)
                return false // Error reading file, consider it not empty
        }

        return true // No non-whitespace content found
}

func isWhitespace(text string) bool {
        for _, char := range text {
                if !unicode.IsSpace(char) {
                        return false
                }
        }
        return true
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
    var userMode bool = false

    // Check if user is root
    if os.Geteuid() != 0 {
        userMode = true
    }

    // Create TmpDir if it doesn't exist
    if _, err := os.Stat(TmpDir); os.IsNotExist(err) {
        err = os.MkdirAll(TmpDir, 0755)
        
        if err != nil {
            fmt.Println("Error creating tmp directory: \n" + TmpDir + "\n" + err.Error())
            os.Exit(1)
        }

    }

    if FileExists(TmpDir + "/monokit.lock") {
        fmt.Println("Monokit is already running, exiting...")
        os.Exit(1)
    }
    
    // Create lockfile
    os.Create(TmpDir + "/monokit.lock")

    LogInit(userMode)
    ConfInit("global", &Config)
}

func WriteToFile(filename string, data string) error {
        file, err := os.Create(filename)
        if err != nil {
                return err
        }
        defer file.Close()

        _, err = file.WriteString(data)
        return err
}

func IsInArray(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

func FileExists(filePath string) bool {
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return false
    }
    return true
}

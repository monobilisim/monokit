package common

import (
    "os"
    "fmt"
    "strconv"
    "github.com/sirupsen/logrus"
)

func LogInit() {
    logrus.SetFormatter(&logrus.JSONFormatter{})
    
    logFile, err := os.OpenFile("/var/log/mono-go.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        panic(err)
    }

    logrus.SetOutput(logFile)

    logrus.SetLevel(logrus.InfoLevel)
    
}

func LogError(err string) {
    fmt.Println(Fail + err + Reset)
    logrus.Error(err)
}

func PrettyPrint(name string, lessOrMore string, value float64, hasPercentage bool) {
    var par string

    if hasPercentage {
        par = "%)"
    } else {
        par = ")"
    }

    fmt.Println(Blue + name + Reset + " is " + lessOrMore + " (" + strconv.FormatFloat(value, 'f', 2, 64) + par + Reset)
}

package common

import (
    "os"
    "fmt"
    "path"
    "runtime"
    "strconv"
    "github.com/sirupsen/logrus"
)

func LogInit() {

    logrus.SetReportCaller(true)
    logrus.SetFormatter(&logrus.JSONFormatter{                                             
            CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {                                                     
            fileName := path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)       
            //return frame.Function, fileName                                        
            return "", fileName                                                      
        },                                                                           
    })

    logFile, err := os.OpenFile("/var/log/monokit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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

func PrettyPrintStr(name string, lessOrMore bool, value string) {
    var color string
    var not string 

    if lessOrMore {
        color = Green
    } else {
        color = Fail
        not = "not "
    }

    fmt.Println(Blue + name + Reset + " is " + not + color + value + Reset)
}

func PrettyPrint(name string, lessOrMore string, value float64, hasPercentage bool, wantFloat bool) {
    var par string
    var floatDepth int

    if hasPercentage {
        par = "%)"
    } else {
        par = ")"
    }

    if wantFloat {
        floatDepth = 2    
    } else {
        floatDepth = 0
    }

    fmt.Println(Blue + name + Reset + " is " + lessOrMore + " (" + strconv.FormatFloat(value, 'f', floatDepth, 64) + par + Reset)
}

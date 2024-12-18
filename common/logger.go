package common

import (
    "os"
    "fmt"
    "path"
    "runtime"
    "strconv"
    "github.com/sirupsen/logrus"
)

func LogInit(userMode bool) {

    logfilePath := "/var/log/monokit.log"

    if userMode {
        xdgStateHome := os.Getenv("XDG_STATE_HOME")
        if xdgStateHome == "" {
            xdgStateHome = os.Getenv("HOME") + "/.local/state"
        }

        // Create the directory if it doesn't exist
        if _, err := os.Stat(xdgStateHome + "/monokit"); os.IsNotExist(err) {
            os.MkdirAll(xdgStateHome + "/monokit", 0755)
        }

        logfilePath = xdgStateHome + "/monokit/monokit.log"
    }

    logrus.SetReportCaller(true)
    logrus.SetFormatter(&logrus.JSONFormatter{                                             
            CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {                                                     
            fileName := path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)       
            //return frame.Function, fileName                                        
            return "", fileName                                                      
        },                                                                           
    })

    logFile, err := os.OpenFile(logfilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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

func PrettyPrint(name string, lessOrMore string, value float64, hasPercentage bool, wantFloat bool, enableLimit bool, limit float64) {
    var par string
    var floatDepth int
    var final string

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
    
    final = Blue + name + Reset 
    
    if enableLimit == false {
        final = final + " is " + lessOrMore + " (" + strconv.FormatFloat(value, 'f', floatDepth, 64) + par + Reset
    } else {
        final = final + " " + lessOrMore
        if limit > value {
            final = final + Green
        } else {
            final = final + Fail
        }

        final = final + strconv.FormatFloat(value, 'f', floatDepth, 64) + "/" + strconv.FormatFloat(limit, 'f', 0, 64) + Reset 
    }

    fmt.Println(final)
}   

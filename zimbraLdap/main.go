package zimbraLdap

import (
    _ "embed"
    "fmt"
    "os/exec"
    "strings"
    "github.com/monobilisim/monokit/common"
)

//go:embed ldap.sh
var script string

func Main() {
    c := exec.Command("bash")
    c.Stdin = strings.NewReader(script)

    b, e := c.Output()
    if e != nil {
        common.LogError(e.Error())
    }
    fmt.Println(string(b))
}


package mysqlHealth

import (
    "os"
    "fmt"
    "os/exec"
    "strings"
    "database/sql"
    "github.com/go-ini/ini"
    "github.com/monobilisim/monokit/common"
    _ "github.com/go-sql-driver/mysql"
)

var Connection *sql.DB

func mariadbOrMysql() string {
    _, err := exec.LookPath("mysql")
    if err != nil {
        return "mariadb"
    }
    return "mysql"
}

func FindMyCnf() []string {
    cmd := exec.Command(mariadbOrMysql() + "d", "--verbose", "--help")
    output, err := cmd.CombinedOutput()
    if err != nil {
        common.LogError("Error running " + mariadbOrMysql() + "d command:" + err.Error())
        return nil
    }

    lines := strings.Split(string(output), "\n")
    foundDefaultOptions := false
    for _, line := range lines {
        if strings.Contains(line, "Default options") {
            foundDefaultOptions = true
            continue
        }

        if foundDefaultOptions {

            return strings.Fields(strings.Replace(line, "~", os.Getenv("HOME"), 1))
        }
    }

    return nil
}

func MyCnf(profile string) (string, error) {
    var host, port, dbname, user, password, socket string
    var found bool

    for _, path := range FindMyCnf() {
        if _, err := os.Stat(path); err == nil {
            cfg, err := ini.LoadSources(ini.LoadOptions{AllowBooleanKeys: true}, path)
            if err != nil {
                return "", fmt.Errorf("error loading config file %s: %w", path, err)
            }

            for _, s := range cfg.Sections() {
                if profile != "" && s.Name() != profile {
                    continue
                }

                found = true

                host = s.Key("host").String()
                port = s.Key("port").String()
                dbname = s.Key("dbname").String()
                user = s.Key("user").String()
                password = s.Key("password").String()
                socket = s.Key("socket").String()

                // Break after finding the first matching profile
                break
            }
        }
    }

    if !found {
        return "", fmt.Errorf("no matching entry found for profile %s", profile)
    }

    if socket != "" {
        return fmt.Sprintf("%s:%s@unix(%s)/%s", user, password, socket, dbname), nil
    }

    return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbname), nil
}

func Connect() {
    connStr, err := MyCnf("client")
    if err != nil {
        common.LogError(err.Error())
        return
    }

    db, err := sql.Open("mysql", connStr)
    if err != nil {
        common.LogError("Error connecting to database: " + err.Error())
        return
    }

    Connection = db
}

func SelectNow() {
    // Simple query to check if the connection is working
    rows, err := Connection.Query("SELECT NOW()")
    defer rows.Close()
    if err != nil {
        common.LogError("Error querying database: " + err.Error())
        common.AlarmCheckDown("now", "Couldn't run a 'SELECT' statement on MySQL")
        common.PrettyPrintStr("MySQL", false, "accessible")
        return
    } else {
        common.AlarmCheckUp("now", "Can run 'SELECT' statements again")
        common.PrettyPrintStr("MySQL", true, "accessible")
    }
}

func CheckProcessCount() {
    rows, err := Connection.Query("SHOW PROCESSLIST")
    defer rows.Close()
    if err != nil {
        common.LogError("Error querying database: " + err.Error())
        common.AlarmCheckDown("processlist", "Couldn't run a 'SHOW PROCESSLIST' statement on MySQL")
        common.PrettyPrintStr("Number of Processes", false, "accessible")
        return
    } else {
        common.AlarmCheckUp("processlist", "Can run 'SHOW PROCESSLIST' statements again")
    }

    // Count the number of processes

    var count int

    for rows.Next() {
        count++
    }

    if count > DbHealthConfig.Mysql.Process_limit {
        common.AlarmCheckDown("processcount", fmt.Sprintf("Number of MySQL processes is over the limit: %d", count))
        common.PrettyPrint("Number of Processes", "", float64(count), false, false, true, float64(DbHealthConfig.Mysql.Process_limit))
    } else {
        common.AlarmCheckUp("processcount", "Number of MySQL processes is under the limit")
        common.PrettyPrint("Number of Processes", "", float64(count), false, false, true, float64(DbHealthConfig.Mysql.Process_limit))
    }
}

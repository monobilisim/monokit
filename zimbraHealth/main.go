//go:build linux
package zimbraHealth

import (
    "io"
    "os"
    "fmt"
    "time"
    "bytes"
    "bufio"
    "regexp"
    "os/exec"
    "strings"
    "net/http"
    "crypto/tls"
    "database/sql"
    "github.com/spf13/cobra"
    _ "github.com/go-sql-driver/mysql"
    "github.com/monobilisim/monokit/common"
    mail "github.com/monobilisim/monokit/common/mail"
)

var MailHealthConfig mail.MailHealth
var MainDB *sql.DB
var MessageDB *sql.DB
var zimbraPath string

func Main(cmd *cobra.Command, args []string) {
    version := "2.0.0"
    common.ScriptName = "zimbraHealth"
    common.TmpDir = common.TmpDir + "zimbraHealth"
    common.Init()
    common.ConfInit("mail", &MailHealthConfig)

    fmt.Println("Zimbra Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))
    
    if common.ProcGrep("install.sh") {
        fmt.Println("Installation is running. Exiting.")
        return
    }
    
    common.SplitSection("Access through IP:")
    CheckIpAccess()

    common.SplitSection("Zimbra Services:")
    CheckZimbraServices()

    common.SplitSection("Zimbra Version:")
    zimbraVer, err := ExecZimbraCommand("zmcontrol -v")
    if err != nil {
        common.LogError("Error getting zimbra version: " + err.Error())
    }
    common.PrettyPrintStr("Zimbra Version", true, zimbraVer)
    
    if MailHealthConfig.Zimbra.Z_Url != "" {
        common.SplitSection("Checking Z-Push:")
        CheckZPush()
    }

    common.SplitSection("Queued Messages:")
    CheckQueuedMessages()
    
    date := time.Now().Format("13:04")
    if date == "01:00" {
        common.SplitSection("SSL Expiration:")
        CheckSSL()
    }
}

func CheckIpAccess() {
    var productName string
    var templateFile string
    var certFile string
    var keyFile string
    var message string = "Hello World!"
    var ipAddress string
    var regexPattern string
    var proxyBlock string
    var output string

    if _, err := os.Stat("/opt/zimbra"); !os.IsNotExist(err) {
        zimbraPath = "/opt/zimbra"
        productName = "zimbra"
    }

    if _, err := os.Stat("/opt/zextras"); !os.IsNotExist(err) {
        zimbraPath = "/opt/zextras"
        productName = "carbonio"
    }

    if zimbraPath == "" {
        fmt.Println("Zimbra not found in opt, aborting.")
        os.Exit(1)
    }

    templateFile = zimbraPath + "/conf/nginx/templates/nginx.conf.web.https.default.template"
    certFile = zimbraPath + "/ssl/" + productName + "/server/server.crt"
    keyFile = zimbraPath + "/ssl/" + productName + "/server/server.key"

    if _, err := os.Stat(templateFile); os.IsNotExist(err) {
        fmt.Println("Nginx template file " + templateFile + " not found, aborting.")
        os.Exit(1)
    }
    

    if _, err := os.Stat(zimbraPath + "/conf/nginx/external_ip.txt"); !os.IsNotExist(err) {
        // Read file
        file, err := os.ReadFile(zimbraPath + "/conf/nginx/external_ip.txt")
        
        if err != nil {
            common.LogError("Error reading external_ip.txt: " + err.Error())
        }

        ipAddress = strings.TrimSpace(string(file))
    } else {
        // Get IP ifconfig.co
        resp, err := http.Get("https://ifconfig.co")
        
        if err != nil {
            common.LogError("Error getting external IP: " + err.Error())
        }

        defer resp.Body.Close()

        respBody, err := io.ReadAll(resp.Body)
        if err != nil {
            common.LogError("Error reading external IP: " + err.Error())
        }

        ipAddress = strings.TrimSpace(string(respBody))
    }

    ipRegex := `\b[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\b`

    re := regexp.MustCompile(ipRegex)

    matches := re.FindAllString(ipAddress, -1)

    if len(matches) == 0 {
        fmt.Println("External IP not found, aborting.")
        os.Exit(1)
    }

    regexPattern = fmt.Sprintf(
	    `(?m)\n?(server\s+?{\n?\s+listen\s+443\s+ssl\s+http2;\n?\s+server_name\n?\s+%s;\n?\s+ssl_certificate\s+%s;\n?\s+ssl_certificate_key\s+%s;\n?\s+location\s+/\s+{\n?\s+return\s+200\s+'%s';\n?\s+}\n?})`,
		ipAddress,
		certFile,
		keyFile,
		message,
	)

    proxyBlock=fmt.Sprintf(`
        server {
            listen                  443 ssl http2;
            server_name             %s;
            ssl_certificate         %s;
            ssl_certificate_key     %s;
            location / {
                    return 200 '%s';
            }
        }`, ipAddress, certFile, keyFile, message)


    // Run regexPattern on templateFile
    file, err := os.ReadFile(templateFile)

    if err != nil {
        common.LogError("Error reading template file: " + err.Error())
    }

    re = regexp.MustCompile(regexPattern)

    matches = re.FindAllString(string(file), -1)

    if len(matches) > 0 {
        output = strings.ReplaceAll(matches[0], "\x00", "\n")
    }

    if output == "" {
        fmt.Println("Adding proxy control block in " + templateFile + " file...")
        file, err := os.OpenFile(templateFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	    if err != nil {
		    fmt.Printf("Error opening file: %v\n", err)
		    return
	    }
	    defer file.Close()

	    // Write the content of proxyBlock to the file
	    if _, err := file.WriteString(proxyBlock + "\n"); err != nil {
		    fmt.Printf("Error writing to file: %v\n", err)
		    return
	    }
        fmt.Println("Proxy control block added to " + templateFile + " file.")
    }

    httpClient := &http.Client{
        Timeout: 10 * time.Second,
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
        },
    }

    req, err := http.NewRequest("GET", "https://" + ipAddress, nil)

    if err != nil {
        common.LogError("Error creating request: " + err.Error())
    }

    _, err = httpClient.Do(req)

    if err != nil {
        common.PrettyPrintStr("Access with IP", false, "accessible")
    } else {
        common.PrettyPrintStr("Access with IP", true, "accessible")
    }
}

func CheckZimbraServices() {
    var zimbraServices []string
    
    status, err := ExecZimbraCommand("zmcontrol status")
    
    if err != nil {
        common.LogError("Error getting zimbra status: " + err.Error())
        return
    }
    
    for _, service := range strings.Split(status, "\n")[1:] {
        svc := strings.Join(strings.Fields(service), " ")
        svcSplit := strings.Split(svc, " ")
        
        if len(svcSplit) < 2 {
            continue
        }
        
        serviceStatus := svcSplit[len(svcSplit)-1]
        serviceName := strings.Join(svcSplit[:len(svcSplit)-1], " ")
        zimbraServices = append(zimbraServices, serviceName)

        if serviceStatus == "Running" {
            common.PrettyPrintStr(serviceName, true, "Running")
            common.AlarmCheckUp(serviceName, serviceName + " is now running", false)
        } else {
            common.PrettyPrintStr(serviceName, false, "Running")
        }
    }
}

func ExecZimbraCommand(command string) (string, error) {
    zimbraUser := "zimbra"

    // Check if zimbra user exists
    cmd := exec.Command("id", "zimbra")
    err := cmd.Run()
    if err != nil {
        zimbraUser = "zextras"
    }

    // Execute command
    cmd := exec.Command("/bin/su", zimbraUser, "-c", zimbraPath + "/bin/" + command)
    
    var out bytes.Buffer
	cmd.Stdout = &out
    cmd.Stderr = os.Stderr
    cmd.Run()

    if cmd.ProcessState.ExitCode() != 0 {
        return "", fmt.Errorf("Command failed: " + command)
    }

    return out.String(), nil
}

func CheckZPush() {
    zpushHeader := false
    
    client := &http.Client{
        Timeout: 10 * time.Second,
    }

    req, err := http.NewRequest("GET", MailHealthConfig.Zimbra.Z_Url, nil)

    if err != nil {
        common.LogError("Error creating request: " + err.Error())
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("Error getting response: " + err.Error())
    } else {
        for key, value := range resp.Header {
            if strings.Contains(strings.ToLower(key), "zpush") || strings.Contains(strings.ToLower(value[0]), "zpush") {
                zpushHeader = true
                break
            }
        }
    }

    if zpushHeader {
        common.PrettyPrintStr("Z-Push", true, "Running")
        common.AlarmCheckUp("zpush", "Z-Push is now running", false)
    } else {
        common.PrettyPrintStr("Z-Push", false, "Running")
        common.AlarmCheckDown("zpush", "Z-Push is not running", false)
    }
}

func CheckQueuedMessages() {
    cmd := exec.Command(zimbraPath + "/common/sbin/mailq")
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running mailq:", err)
		return
	}

	// Regex to match lines starting with A-F or 0-9
	re := regexp.MustCompile(`^[A-F0-9]`)

	// Count matching lines
	scanner := bufio.NewScanner(&out)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading mailq output:", err)
		return
	}

    common.PrettyPrint("Queued Messages", "", float64(count), false, false, true, float64(MailHealthConfig.Zimbra.Queue_Limit))

    if count > MailHealthConfig.Zimbra.Queue_Limit {
        common.AlarmCheckDown("mailq", "Mail queue is over the limit", false)
    } else {
        common.AlarmCheckUp("mailq", "Mail queue is under the limit", false)
    }
}

func CheckSSL() {
    var mailHost string
    zmHostname, err := ExecZimbraCommand("zmhostname")
    if err != nil {
        common.LogError("Error getting zimbra hostname: " + err.Error())
    }
    mailHost1, err := ExecZimbraCommand("zmprov gs " + zmHostname)
    if err != nil {
        common.LogError("Error getting mail host: " + err.Error())
    }
    for _, mailHost1 := range strings.Split(mailHost1, "\n") {
        if strings.Contains(mailHost1, "zimbraServiceHostname: ") {
            mailHost = strings.Split(mailHost1, "zimbraServiceHostname: ")[1]
            break
        }
    }

    if mailHost == "" {
        common.LogError("Mail host not found")
    }
    
    conn, err := tls.Dial("tcp", mailHost + ":443", &tls.Config{InsecureSkipVerify: true})

    if err != nil {
        common.LogError("Error connecting to mail host: " + err.Error())
    }
    defer conn.Close()

    certs := conn.ConnectionState().PeerCertificates
    if len(certs) == 0 {
        common.LogError("No certificates found")
    }
    
    cert := certs[0]

    // Get days until notAfter
    days := int(cert.NotAfter.Sub(time.Now()).Hours() / 24)
    if days < 10 {
        common.PrettyPrintStr("SSL Certificate", true, fmt.Sprintf("expiring in %d days", days))
        common.AlarmCheckDown("sslcert", "SSL Certificate is expiring in " + fmt.Sprintf("%d days", days), false)
    } else {
        common.PrettyPrintStr("SSL Certificate", true, fmt.Sprintf("expiring in %d days", days))
        common.AlarmCheckUp("sslcert", "SSL Certificate is expiring in " + fmt.Sprintf("%d days", days), false)
    }
}

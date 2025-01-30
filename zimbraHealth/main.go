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
    "io/ioutil"
    "crypto/tls"
    "github.com/spf13/cobra"
    "github.com/monobilisim/monokit/common"
    mail "github.com/monobilisim/monokit/common/mail"
    issues "github.com/monobilisim/monokit/common/redmine/issues"
)

var MailHealthConfig mail.MailHealth
var zimbraPath string
var restartCounter int
var templateFile string

func Main(cmd *cobra.Command, args []string) {
    version := "2.2.1"
    common.ScriptName = "zimbraHealth"
    common.TmpDir = common.TmpDir + "zimbraHealth"
    common.Init()
    common.ConfInit("mail", &MailHealthConfig)

    fmt.Println("Zimbra Health Check REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))
    
    if common.ProcGrep("install.sh", true) {
        fmt.Println("Installation is running. Exiting.")
        return
    }
    
    common.SplitSection("Access through IP:")
    CheckIpAccess()

    common.SplitSection("Zimbra Services:")
    CheckZimbraServices()

    common.SplitSection("Zimbra Version:")
    zimbraVer, err := ExecZimbraCommand("zmcontrol -v", false, false)
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
    
    if !common.IsEmptyOrWhitespaceStr(MailHealthConfig.Zimbra.Webhook_tail.Logfile) && !common.IsEmptyOrWhitespaceStr(MailHealthConfig.Zimbra.Webhook_tail.Filter) {
        TailWebhook(MailHealthConfig.Zimbra.Webhook_tail.Logfile, MailHealthConfig.Zimbra.Webhook_tail.Filter)
    }

    date := time.Now().Format("13:04")
    if date == "01:00" {
        common.SplitSection("SSL Expiration:")
        CheckSSL()

        Zmfixperms()
    }
}


func escapeJSON(input string) string {
	output := bytes.Buffer{}
	for _, r := range input {
		switch r {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		default:
			output.WriteRune(r)
		}
	}
	return output.String()
}

func TailWebhook(filePath string, pattern string) {
    // Compile the regex pattern
	regex, err := regexp.Compile(pattern)
	if err != nil {
        common.LogError("Invalid regex pattern: " + err.Error())
    }
	
    // Open the file
	file, err := os.Open(filePath)
	if err != nil {
        common.LogError("Error opening file: " + err.Error())
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if regex.MatchString(line) {
            // Use regex to get the ID from the log line
            // `- (\d+)\]` matches the ID in the log line
            re := regexp.MustCompile(`- (\d+)\]`)
            matches := re.FindStringSubmatch(line)
            if len(matches) < 2 {
                common.LogError("Error matching ID in log line: " + line)
                return
            }
            id := matches[1]
            
            // Check if the file exists
            if _, err := os.Stat(common.TmpDir + "/webhook_tail_" + id); os.IsNotExist(err) {
                // Create the file
                os.Create(common.TmpDir + "/webhook_tail_" + id)
                
                // Send the alarm
                common.Alarm("[zimbraHealth - " + common.Config.Identifier + "] Webhook tail matched: " + escapeJSON(line), MailHealthConfig.Zimbra.Webhook_tail.Stream, MailHealthConfig.Zimbra.Webhook_tail.Topic, true)
            }
        }
	}

	if err := scanner.Err(); err != nil {
        common.LogError("Error reading file: " + err.Error())
	}
}    


func Zmfixperms() {
    // Run zmfixperms
    _, _ = ExecZimbraCommand("libexec/zmfixperms", true, true)
}

func CheckIpAccess() {
    var productName string
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
	    `(?m)\n?(server\s+?{\n?\s+listen\s+443\s+ssl\s+http2;\n?\s+server_name\n?\s+%s;\n?\s+ssl_certificate\s+%s;\n?\s+ssl_certificate_key\s+%s;\n?\s+location\s+\/\s+{\n?\s+return\s+200\s+'%s';\n?\s+}\n?})`,
		ipAddress,
		certFile,
		keyFile,
		message,
	)

    proxyBlock=fmt.Sprintf(`server {
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
        common.PrettyPrintStr("Access with IP", false, "accessible")
        common.AlarmCheckDown("accesswithip", "Can't access to the IP at all: " + ipAddress + " - " + err.Error(), false, "", "")
        return
    }

    resp, err := httpClient.Do(req)

    if err != nil {
        common.PrettyPrintStr("Access with IP", false, "accessible")
        common.AlarmCheckDown("accesswithip", "Can't access to the IP at all: " + ipAddress + " - " + err.Error(), false, "", "")
        return
    }


    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)

    bodyStr := string(body)

    if err != nil {
        common.LogError("Error getting response: " + err.Error())
        common.PrettyPrintStr("Access with IP", false, "accessible")
        common.AlarmCheckDown("accesswithip", "Can't access to the IP at all: " + ipAddress + " - " + err.Error(), false, "", "")
        return
    }

    if ! strings.Contains(bodyStr, message) {
        common.PrettyPrintStr("Access with IP", true, "accessible")
        common.AlarmCheckDown("accesswithip", "Can access to zimbra through plain IP: " + ipAddress, false, "", "")
    } else {
        common.PrettyPrintStr("Access with IP", true, "not accessible")
        common.AlarmCheckUp("accesswithip", "Can't access to zimbra through plain IP: " + ipAddress, false)
    }
}

func RestartZimbraService(service string) {
    if restartCounter > MailHealthConfig.Zimbra.Restart_Limit {
        common.AlarmCheckDown(service, "Restart limit reached for " + service, false, "", "")
        return
    }
    
    _, err := ExecZimbraCommand("zmcontrol start", false, false)
    
    if err != nil {
        common.LogError("Error starting Zimbra service " + service + ": " + err.Error())
        return
    }

    restartCounter++

    CheckZimbraServices()
}

func CheckZimbraServices() {
    var zimbraServices []string
    
    status, _ := ExecZimbraCommand("zmcontrol status", false, false)

    for _, service := range strings.Split(status, "\n")[1:] {
        svc := strings.Join(strings.Fields(service), " ")
        svcSplit := strings.Split(strings.ToLower(svc), "running")
        
        if len(svcSplit) < 2 {
            continue
        }

        var serviceName string
        var serviceStatus string
        
        if strings.Contains(svc, "is not running") {
            serviceName = strings.TrimSpace(strings.ReplaceAll(svcSplit[0], "is not", ""))
            serviceStatus = "Not Running"
        } else {
            serviceName = strings.TrimSpace(svcSplit[0])
            serviceStatus = "Running"
        }

        zimbraServices = append(zimbraServices, serviceName)

        if serviceStatus == "Running" {
            common.PrettyPrintStr(serviceName, true, "Running")
            common.AlarmCheckUp(serviceName, serviceName + " is now running", false)
        } else {
            common.PrettyPrintStr(serviceName, false, "Running")
            common.AlarmCheckDown(serviceName, serviceName + " is not running", false, "", "")
            if MailHealthConfig.Zimbra.Restart {
                RestartZimbraService(serviceName)
            }
        }
    }
}

func changeImmutable(filePath string, add bool) {
	flag := "+i"
	if !add {
		flag = "-i"
	}
	cmd := exec.Command("chattr", flag, filePath)
	err := cmd.Run()
	if err != nil {
        common.LogError("Error changing file attributes: " + err.Error())
	}
}

func modifyFile(templateFile string) {
	// Read the file content
	content, err := ioutil.ReadFile(templateFile)
	if err != nil {
	    common.LogError("Error reading file: " + err.Error())
    }

	text := string(content)

    if strings.Contains(text, "nginx-php-fpm.conf") {
        return
    }

	// Define regex patterns and replacements
	blockRegex := regexp.MustCompile(`(?s)(Microsoft-Server-ActiveSync.*?# For audit)`)
	modifiedBlock := blockRegex.ReplaceAllStringFunc(text, func(match string) string {
		match = regexp.MustCompile(`proxy_pass`).ReplaceAllString(match, "### proxy_pass")
		match = regexp.MustCompile(`proxy_read_timeout`).ReplaceAllString(match, "### proxy_read_timeout")
		match = regexp.MustCompile(`proxy_buffering`).ReplaceAllString(match, "### proxy_buffering")
		return regexp.MustCompile(`# For audit`).ReplaceAllString(match, `# Z-PUSH start
        include /etc/nginx-php-fpm.conf;
        # Z-PUSH end

        # For audit`)
	})

	// Write the modified content back to the file
	if err := ioutil.WriteFile(templateFile, []byte(modifiedBlock), 0644); err != nil {
	    common.LogError("Error writing to file: " + err.Error())
    }


    fmt.Println("Added Z-Push block to " + templateFile + " file, restarting zimbra proxy service...")
    _, err = ExecZimbraCommand("zmproxyctl restart", false, false)
    if err != nil {
        common.LogError("Error restarting zimbra proxy service: " + err.Error())
    }
}

func ExecZimbraCommand(command string, fullPath bool, runAsRoot bool) (string, error) {
    zimbraUser := "zimbra"

    // Check if zimbra user exists
    cmd := exec.Command("id", "zimbra")
    err := cmd.Run()
    if err != nil {
        zimbraUser = "zextras"
    }

    if runAsRoot {
        zimbraUser = "root"
    }
   
    cmd = nil

    // Execute command
    if fullPath {
        cmd = exec.Command("/bin/su", zimbraUser, "-c", zimbraPath + "/" + command)
    } else {
        cmd = exec.Command("/bin/su", zimbraUser, "-c", zimbraPath + "/bin/" + command)
    }
    
    var out bytes.Buffer
	cmd.Stdout = &out
    cmd.Stderr = os.Stderr
    cmd.Run()

    if cmd.ProcessState.ExitCode() != 0 {
        return out.String(), fmt.Errorf("Command failed: " + command + " with stdout: " + out.String())
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
        common.AlarmCheckDown("zpush", "Z-Push is not running", false, "", "")
    }

    // Check if /etc/nginx-php-fpm.conf exists
    if _, err := os.Stat("/etc/nginx-php-fpm.conf"); os.IsNotExist(err) {
        common.PrettyPrintStr("Z-Push Nginx Config file", false, "found")
    } else {
        common.PrettyPrintStr("Z-Push Nginx Config file", true, "found")
        modifyFile(templateFile)
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
        common.AlarmCheckDown("mailq", "Mail queue is over the limit", false, "", "")
    } else {
        common.AlarmCheckUp("mailq", "Mail queue is under the limit", false)
    }
}

func CheckSSL() {
    var mailHost string
    zmHostname, err := ExecZimbraCommand("zmhostname", false, false)
    if err != nil {
        common.LogError("Error getting zimbra hostname: " + err.Error())
    }
    mailHost1, err := ExecZimbraCommand("zmprov gs " + zmHostname, false, false)
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
        common.AlarmCheckDown("sslcert", "SSL Certificate is expiring in " + fmt.Sprintf("%d days", days), false, "", "")
        viewDeployedCert, _ := ExecZimbraCommand("zmcertmgr viewdeployedcrt", false, false)
        issues.CheckDown("sslcert", common.Config.Identifier + " sunucusunun SSL sertifikası bitimine " + fmt.Sprintf("%d gün kaldı", days), "```json\n" + viewDeployedCert + "\n```", false, 0)
    } else {
        common.PrettyPrintStr("SSL Certificate", true, fmt.Sprintf("expiring in %d days", days))
        common.AlarmCheckUp("sslcert", "SSL Certificate is expiring in " + fmt.Sprintf("%d days", days), false)
        issues.CheckUp("sslcert", "SSL  sertifikası artık " + fmt.Sprintf("%d gün sonra sona erecek şekilde güncellendi", days))
    }
}

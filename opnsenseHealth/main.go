package opnsenseHealth

import (
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type opnsenseXMLConfig struct {
	XMLName xml.Name `xml:"opnsense"`
	System  struct {
		Hostname string `xml:"hostname"`
		Domain   string `xml:"domain"`
	} `xml:"system"`
}

func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "opnsenseHealth"
	common.TmpDir = common.TmpDir + "opnsenseHealth"
	common.Init()
	common.ConfInit("opnsense", &OpnsenseHealthConfig)

	common.WrapperGetServiceStatus("opnsenseHealth")

	if OpnsenseHealthConfig.ExpireDays == 0 {
		OpnsenseHealthConfig.ExpireDays = 7
	}

	healthData := collectOpnsenseHealthData()

	if err := common.PostHostHealth("opnsenseHealth", healthData); err != nil {
		log.Error().Err(err).Msg("opnsenseHealth: failed to POST health data")
	}

	displayBoxUI(healthData)
}

func getOpnsenseDomain() string {
	xmlFile, err := os.Open("/conf/config.xml")
	if err != nil {
		log.Warn().Err(err).Msg("Could not open /conf/config.xml, using empty ServerName for SNI")
		return ""
	}
	defer xmlFile.Close()

	byteValue, err := io.ReadAll(xmlFile)
	if err != nil {
		log.Warn().Err(err).Msg("Could not read /conf/config.xml")
		return ""
	}

	var config opnsenseXMLConfig
	if err := xml.Unmarshal(byteValue, &config); err != nil {
		log.Warn().Err(err).Msg("Could not parse /conf/config.xml")
		return ""
	}

	if config.System.Hostname != "" && config.System.Domain != "" {
		return config.System.Hostname + "." + config.System.Domain
	} else if config.System.Hostname != "" {
		return config.System.Hostname
	}
	return ""
}

func collectOpnsenseHealthData() *OpnsenseHealthData {
	data := &OpnsenseHealthData{
		Status: "Checking",
	}

	domainName := getOpnsenseDomain()
	targetHost, err := os.Hostname()
	if err != nil || targetHost == "" {
		targetHost = "127.0.0.1"
	}

	portsToTry := []int{OpnsenseHealthConfig.Port}
	if OpnsenseHealthConfig.Port == 0 {
		portsToTry = []int{9443, 443}
	}

	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	if domainName != "" {
		conf.ServerName = domainName
	}

	var conn *tls.Conn
	var address string
	var lastErr error

	for _, p := range portsToTry {
		portStr := strconv.Itoa(p)
		address = net.JoinHostPort(targetHost, portStr)
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", address, conf)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
	}

	if lastErr != nil {
		data.Status = "Connection Failed"
		msg := fmt.Sprintf("Could not connect to %s over TLS: %v", address, lastErr)
		log.Error().Err(lastErr).Msg(msg)

		common.AlarmCheckDown("opnsense_ssl", msg, false, "", "")
		issues.CheckDown("opnsense_ssl", common.Config.Identifier+" için OPNsense SSL bağlantı hatası", msg, false, 0)
		return data
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		data.Status = "No Certificate Found"
		msg := "No SSL certificate found on " + address
		common.AlarmCheckDown("opnsense_ssl", msg, false, "", "")
		issues.CheckDown("opnsense_ssl", common.Config.Identifier+" için OPNsense SSL sertifikası bulunamadı", msg, false, 0)
		return data
	}

	cert := certs[0]
	data.Subject = cert.Subject.String()
	if cert.Subject.CommonName != "" {
		data.Subject = cert.Subject.CommonName
	}
	data.Issuer = cert.Issuer.String()
	if cert.Issuer.CommonName != "" {
		data.Issuer = cert.Issuer.CommonName
	}
	data.ExpiryDate = cert.NotAfter.Format("2006-01-02 15:04:05")

	now := time.Now()
	remaining := cert.NotAfter.Sub(now)
	daysRemaining := int(remaining.Hours() / 24)
	data.DaysRemaining = daysRemaining

	issueMsg := fmt.Sprintf("Sertifika: %s\nVeren: %s\nBitiş Tarihi: %s\nKalan Gün: %d", data.Subject, data.Issuer, data.ExpiryDate, data.DaysRemaining)
	alarmMsg := fmt.Sprintf("Certificate: %s\nIssuer: %s\nExpiry Date: %s\nDays Remaining: %d", data.Subject, data.Issuer, data.ExpiryDate, data.DaysRemaining)

	if remaining <= 0 {
		data.Status = "Expired"
		common.AlarmCheckDown("opnsense_ssl", "SSL Certificate Expired!\n\n"+alarmMsg, false, "", "")
		issues.CheckDown("opnsense_ssl", common.Config.Identifier+" için OPNsense SSL sertifikası süresi doldu", issueMsg, false, 0)

		id := issues.Show("opnsense_ssl")
		if id != "" {
			common.AlarmCheckUp("opnsense_ssl_redmineissue", "Redmine issue exists for SSL expiry", false)
			common.AlarmCheckDown("opnsense_ssl", "SSL Certificate Expired!\n\n"+alarmMsg+"\n\nRedmine Issue: "+common.GetRedmineDisplayUrl()+"/issues/"+id, false, "", "")
		} else {
			log.Debug().Msg("issues.Show(\"opnsense_ssl\") returned empty.")
		}
	} else if daysRemaining <= OpnsenseHealthConfig.ExpireDays {
		data.Status = "Expiring Soon"
		common.AlarmCheckDown("opnsense_ssl", "SSL Certificate Expiring Soon!\n\n"+alarmMsg, false, "", "")
		issues.CheckDown("opnsense_ssl", common.Config.Identifier+" için OPNsense SSL sertifikası yakında süresi dolacak", issueMsg, false, 0)

		id := issues.Show("opnsense_ssl")
		if id != "" {
			common.AlarmCheckUp("opnsense_ssl_redmineissue", "Redmine issue exists for SSL expiry", false)
			common.AlarmCheckDown("opnsense_ssl", "SSL Certificate Expiring Soon!\n\n"+alarmMsg+"\n\nRedmine Issue: "+common.GetRedmineDisplayUrl()+"/issues/"+id, false, "", "")
		} else {
			log.Debug().Msg("issues.Show(\"opnsense_ssl\") returned empty.")
		}
	} else {
		data.Status = "Valid"
		common.AlarmCheckUp("opnsense_ssl", "SSL Certificate is valid.\n\n"+alarmMsg, false)
		issues.CheckUp("opnsense_ssl", common.Config.Identifier+" için OPNsense SSL sertifikası geçerli.")
		common.AlarmCheckUp("opnsense_ssl_redmineissue", "SSL Certificate is valid, clearing any Redmine issue creation failure alarm", false)
	}

	return data
}

func displayBoxUI(data *OpnsenseHealthData) {
	title := "monokit opnsenseHealth"
	content := RenderOpnsenseHealthCLI(data)
	renderedBox := common.DisplayBox(title, content)
	fmt.Println(renderedBox)
}

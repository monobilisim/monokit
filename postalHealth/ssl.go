//go:build linux

package postalHealth

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"time"

	"github.com/monobilisim/monokit/common"
	issue "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
)

// evalExpiry is a pure helper (no network) so it's unit-testable.
func evalExpiry(notAfter time.Time, thresholdDays int) (daysLeft int, expiringSoon bool) {
	daysLeft = int(time.Until(notAfter).Hours() / 24)
	expiringSoon = daysLeft < thresholdDays
	return
}

// CheckSSL connects to the Postal SMTP port via STARTTLS and checks certificate expiry.
func CheckSSL(skipOutput bool) SSLCertInfo {
	host := MailHealthConfig.Postal.Ssl.Host
	if host == "" {
		host = "localhost"
	}
	port := MailHealthConfig.Postal.Ssl.Port
	if port == 0 {
		port = 25
	}
	threshold := MailHealthConfig.Postal.Ssl.Expiry_Threshold_Days
	if threshold == 0 {
		threshold = 10
	}

	info := SSLCertInfo{Host: host, Port: port}
	addr := fmt.Sprintf("%s:%d", host, port)

	client, err := smtp.Dial(addr)
	if err != nil {
		info.Message = "Error connecting to " + addr + " for SSL check: " + err.Error()
		log.Error().Msg(info.Message)
		common.AlarmCheckDown("postal_sslcert_conn", info.Message, false, "", "")
		return info
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		info.Message = "Error during STARTTLS to " + addr + ": " + err.Error()
		log.Error().Msg(info.Message)
		common.AlarmCheckDown("postal_sslcert_conn", info.Message, false, "", "")
		return info
	}
	common.AlarmCheckUp("postal_sslcert_conn", "Connected to "+addr+" for SSL check", false)

	state, ok := client.TLSConnectionState()
	if !ok || len(state.PeerCertificates) == 0 {
		info.Message = "No SSL certificate found for " + addr
		log.Error().Msg(info.Message)
		common.AlarmCheckDown("postal_sslcert_nocert", info.Message, false, "", "")
		return info
	}

	cert := state.PeerCertificates[0]
	info.CheckStatus = true
	info.DaysUntilExpiry, info.ExpiringSoon = evalExpiry(cert.NotAfter, threshold)

	alarmMsg := fmt.Sprintf("SSL Certificate for %s expires in %d days", addr, info.DaysUntilExpiry)
	subject := common.Config.Identifier + " sunucusunun " + addr + " SMTP SSL sertifikası bitimine yaklaşıyor"

	if info.ExpiringSoon {
		log.Warn().Msg("Postal SMTP SSL certificate is expiring soon.")
		common.AlarmCheckDown("postal_sslcert", alarmMsg, false, "", "")
		issueBody := fmt.Sprintf("Sertifika %s için %s tarihinde sona eriyor (%d gün kaldı).", addr, cert.NotAfter.Format("2006-01-02"), info.DaysUntilExpiry)
		issue.CheckDown("postal_sslcert", subject, issueBody, false, 0)
	} else {
		common.AlarmCheckUp("postal_sslcert", alarmMsg, false)
		issue.CheckUp("postal_sslcert", fmt.Sprintf("SSL sertifikası artık %d gün sonra sona erecek şekilde güncellendi", info.DaysUntilExpiry))
	}

	return info
}

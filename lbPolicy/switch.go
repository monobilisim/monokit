package lbPolicy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/itchyny/gojq"
	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

var (
	noChangesCounter int64

	// Tek HTTP client: bağlantı havuzu (keep-alive) tüm isteklerde paylaşılır,
	// böylece her istekte TLS handshake + yeni bağlantı maliyeti ödenmez.
	sharedHTTPClientOnce sync.Once
	sharedHTTPClient     *http.Client

	// Aynı çalıştırma içinde her Caddy API URL'i için /config/apps/http/servers
	// yanıtı yalnızca bir kez çekilir; vhost başına tekrar tekrar indirilmez.
	// Eşzamanlı API URL worker'ları eriştiği için mutex ile korunur.
	serversConfigCacheMu sync.RWMutex
	serversConfigCache   = map[string]map[string]interface{}{}

	// Her Caddy config değişikliği tüm modülleri yeniden provision eder (ölçüldü:
	// yavaş bir modülle ~142 sn). vhost başına PATCH atmak N reload demektir;
	// bu yüzden değişiklikler LB başına toplanır ve tek /load ile uygulanır.
	pendingMu     sync.Mutex
	pendingRoutes = map[string][]stagedRouteChange{}
)

// Bir LB için bekleyen rota değişikliği.
type stagedRouteChange struct {
	server string
	index  int
	route  map[string]interface{}
	line   string
}

// /load tüm config'i yükler ve yavaş olabilir; okuma ve yükleme için ayrı,
// uzun timeout'lu client kullanılır.
const loadTimeout = 300 * time.Second

var (
	loadHTTPClientOnce sync.Once
	loadHTTPClient     *http.Client
)

func loadClient() *http.Client {
	loadHTTPClientOnce.Do(func() {
		c := newHTTPClient()
		c.Timeout = loadTimeout
		loadHTTPClient = c
	})
	return loadHTTPClient
}

func stageRouteChange(actualUrl string, change stagedRouteChange) {
	pendingMu.Lock()
	pendingRoutes[actualUrl] = append(pendingRoutes[actualUrl], change)
	pendingMu.Unlock()
}

func resetPendingRoutes() {
	pendingMu.Lock()
	pendingRoutes = map[string][]stagedRouteChange{}
	pendingMu.Unlock()
}

func applyBasicAuth(req *http.Request, usernamePassword string) error {
	if usernamePassword == "" {
		return nil
	}
	credentials := strings.SplitN(usernamePassword, ":", 2)
	if len(credentials) != 2 {
		return fmt.Errorf("invalid usernamePassword format, expected 'username:password'")
	}
	req.SetBasicAuth(credentials[0], credentials[1])
	return nil
}

func configServers(cfg map[string]interface{}) (map[string]interface{}, error) {
	apps, ok := cfg["apps"].(map[string]interface{})
	if !ok {
		return nil, errors.New("config has no apps object")
	}
	httpApp, ok := apps["http"].(map[string]interface{})
	if !ok {
		return nil, errors.New("config has no apps.http object")
	}
	servers, ok := httpApp["servers"].(map[string]interface{})
	if !ok {
		return nil, errors.New("config has no apps.http.servers object")
	}
	return servers, nil
}

func fetchFullConfig(actualUrl string, usernamePassword string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", actualUrl+"/config/", nil)
	if err != nil {
		return nil, err
	}
	if err := applyBasicAuth(req, usernamePassword); err != nil {
		return nil, err
	}

	resp, err := doWithRetry(loadClient(), req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s returned %s; body=%s", actualUrl+"/config/", resp.Status, string(b))
	}

	var cfg map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadFullConfig(cfg map[string]interface{}, actualUrl string, usernamePassword string) error {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, actualUrl+"/load", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := applyBasicAuth(req, usernamePassword); err != nil {
		return err
	}

	// Tek deneme: başarısız bir /load'u tekrarlamak yeni bir reload demektir.
	resp, err := doWithRetry(loadClient(), req, 1)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s returned %s; body=%s", actualUrl+"/load", resp.Status, string(b))
	}
	return nil
}

// ApplyPendingChanges, bir LB için toplanan rotaları tek bir config
// yüklemesiyle uygular: tam config çekilir, rotalar yerine konur, /load edilir.
func ApplyPendingChanges(actualUrl string, usernamePassword string) error {
	pendingMu.Lock()
	changes := pendingRoutes[actualUrl]
	delete(pendingRoutes, actualUrl)
	pendingMu.Unlock()

	if len(changes) == 0 {
		return nil
	}

	cfg, err := fetchFullConfig(actualUrl, usernamePassword)
	if err != nil {
		return err
	}

	servers, err := configServers(cfg)
	if err != nil {
		return err
	}

	applied := make([]stagedRouteChange, 0, len(changes))
	for _, change := range changes {
		srv, ok := servers[change.server].(map[string]interface{})
		if !ok {
			return fmt.Errorf("server %s is missing in the config of %s", change.server, actualUrl)
		}
		routes, ok := srv["routes"].([]interface{})
		if !ok {
			return fmt.Errorf("server %s has no routes array in the config of %s", change.server, actualUrl)
		}
		if change.index < 0 || change.index >= len(routes) {
			return fmt.Errorf("route index %d is out of range for server %s in the config of %s", change.index, change.server, actualUrl)
		}
		routes[change.index] = change.route
		applied = append(applied, change)
	}

	if err := loadFullConfig(cfg, actualUrl, usernamePassword); err != nil {
		return err
	}

	for _, change := range applied {
		fmt.Println(change.line)
	}
	fmt.Println("Applied " + strconv.Itoa(len(applied)) + " route change(s) to " + actualUrl + " in one config load")
	return nil
}

func newHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 128,
		IdleConnTimeout:     90 * time.Second,

		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}

func sharedClient() *http.Client {
	sharedHTTPClientOnce.Do(func() { sharedHTTPClient = newHTTPClient() })
	return sharedHTTPClient
}

func fetchServersConfig(actualUrl string, usernamePassword string) (map[string]interface{}, error) {
	serversConfigCacheMu.RLock()
	data, ok := serversConfigCache[actualUrl]
	serversConfigCacheMu.RUnlock()
	if ok {
		return data, nil
	}

	req, err := http.NewRequest("GET", actualUrl+"/config/apps/http/servers", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(strings.Split(usernamePassword, ":")[0], strings.Split(usernamePassword, ":")[1])

	resp, err := doWithRetry(sharedClient(), req, 5)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s returned %s; body=%s", actualUrl+"/config/apps/http/servers", resp.Status, string(b))
	}

	var respBodyJson map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respBodyJson); err != nil {
		return nil, err
	}

	serversConfigCacheMu.Lock()
	serversConfigCache[actualUrl] = respBodyJson
	serversConfigCacheMu.Unlock()
	return respBodyJson, nil
}

func doWithRetry(client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	backoff := 200 * time.Millisecond
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Body'li isteklerde (PATCH/POST/PUT) her deneme öncesi body'yi tazele
		if attempt > 0 && req.GetBody != nil {
			if rc, gerr := req.GetBody(); gerr == nil {
				req.Body = rc
			} else {
				log.Debug().Str("component", "lbPolicy").Str("operation", "doWithRetry").Str("action", "getbody_failed").Msg(gerr.Error())
			}
		}

		start := time.Now()
		resp, err = client.Do(req)

		if err != nil {
			log.Debug().
				Str("component", "lbPolicy").Str("operation", "doWithRetry").Str("action", "attempt_error").
				Int("attempt", attempt+1).Err(err).Msg("request failed")

			// Timeout'ta tekrar denemek, yavaş bir admin API'ye (reload kuyruğuna)
			// yeni istek eklemekten başka işe yaramaz; istek sunucuda sürüyor olabilir.
			if os.IsTimeout(err) {
				return nil, err
			}
			if attempt == maxRetries-1 {
				return nil, err
			}
			time.Sleep(backoff)
			if backoff < 2*time.Second {
				backoff *= 2
			}
			continue
		}
		if resp.StatusCode < 500 {
			log.Debug().
				Str("component", "lbPolicy").Str("operation", "doWithRetry").Str("action", "attempt_done").
				Int("attempt", attempt+1).Str("status", resp.Status).Dur("latency", time.Since(start)).Msg("request completed")
			return resp, nil
		}

		// 5xx: retry
		log.Debug().
			Str("component", "lbPolicy").Str("operation", "doWithRetry").Str("action", "attempt_5xx").
			Int("attempt", attempt+1).Str("status", resp.Status).Dur("latency", time.Since(start)).Msg("server returned 5xx; will retry")

		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if attempt == maxRetries-1 {
			return resp, nil
		}
		time.Sleep(backoff)
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}

	return resp, err
}

func AlarmCustom(msgType string, message string) {
	customStream := false
	if Config.Alarm.Stream != "" && Config.Alarm.Topic != "" {
		customStream = true
	}
	common.Alarm("[lbPolicy - "+common.Config.Identifier+"] [:"+msgType+":] "+message, Config.Alarm.Stream, Config.Alarm.Topic, customStream)
}

func hostnameToURL(hostname string) (string, error) {
	parts := strings.Split(hostname, "-")
	if len(parts) < 3 {
		return "", errors.New("invalid hostname format")
	}
	domainPart := parts[0]
	envPart := parts[1]
	lbPart := parts[2]
	url := fmt.Sprintf("https://api.%s.%s.%s.biz.tr", lbPart, envPart, domainPart)
	return url, nil
}

func extractHostname(url string) (string, error) {
	var resp *http.Response
	var err error
	maxRetries := 2
	for i := 0; i <= maxRetries; i++ {
		resp, err = sharedClient().Get(url)
		if err == nil {
			break
		}
		fmt.Println("Retrying " + url)
	}
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var hostname string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Hostname:") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				hostname = parts[1]
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if hostname != "" {
		return hostname, nil
	}
	return "", errors.New("hostname not found")
}

func removePassword(urls []string) []string {
	var censoredCaddyAPIUrls []string
	for _, url := range urls {
		if idx := strings.Index(url, ";"); idx != -1 {
			censoredCaddyAPIUrls = append(censoredCaddyAPIUrls, url[idx+1:])
		} else {
			censoredCaddyAPIUrls = append(censoredCaddyAPIUrls, url)
		}
	}
	return censoredCaddyAPIUrls
}

func uniqueSorted(input []string) []string {
	sort.Strings(input)
	var unique []string
	for i, val := range input {
		if i == 0 || val != input[i-1] {
			unique = append(unique, val)
		}
	}
	return unique
}

func AdjustApiUrls() {
	var caddyApiUrlsNew []string
	for _, lbUrl := range Config.Caddy.Lb_Urls {
		log.Debug().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "checking_lb_url").Msg("Checking " + lbUrl)

		hostname, err := extractHostname(lbUrl)
		if err != nil {
			log.Error().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "extract_hostname").Msg(err.Error())
			continue
		}
		urlNew, err := hostnameToURL(hostname)
		if err != nil {
			log.Error().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "hostname_to_url").Msg(err.Error())
			continue
		}

		for _, server := range Config.Caddy.Api_Urls {
			log.Debug().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "checking_server").Msg("Checking " + server + " under " + lbUrl)

			url := strings.Split(server, "@")[1]
			if urlNew == url {
				fmt.Println(urlNew + " is the same as URL, adding to caddyApiUrlsNew")
				caddyApiUrlsNew = append(caddyApiUrlsNew, server)
			}
		}
	}
	for _, urlUp := range Config.Caddy.Api_Urls {
		log.Debug().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "checking_url").Msg("Checking " + urlUp)
		caddyApiUrlsNew = append(caddyApiUrlsNew, urlUp)
	}
	Config.Caddy.Api_Urls = uniqueSorted(caddyApiUrlsNew)
	log.Debug().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "final_urls").Msg("Final caddyApiUrls: " + strings.Join(Config.Caddy.Api_Urls, ", "))
}

func SwitchMain(server string) {
	var CensoredApiUrls []string

	serversConfigCacheMu.Lock()
	serversConfigCache = map[string]map[string]interface{}{}
	resetPendingRoutes()
	serversConfigCacheMu.Unlock()

	if Config.Caddy.Loop_Order == "" {
		Config.Caddy.Loop_Order = "API_URLS"
	}
	if Config.Caddy.Api_Urls == nil || len(Config.Caddy.Api_Urls) == 0 {
		log.Error().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "validation").Msg("Api_Urls is not defined in caddy config")
		os.Exit(1)
	}
	if Config.Caddy.Servers == nil || len(Config.Caddy.Servers) == 0 {
		log.Error().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "validation").Msg("Servers is not defined in caddy config")
		os.Exit(1)
	}
	if (Config.Caddy.Lb_Urls == nil || len(Config.Caddy.Lb_Urls) == 0) && Config.Caddy.Dynamic_Api_Urls {
		log.Error().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "validation").Msg("Lb_Urls is not defined in caddy config, but Dynamic_Api_Urls is enabled")
		os.Exit(1)
	}
	if Config.Caddy.Dynamic_Api_Urls {
		AdjustApiUrls()
		CensoredApiUrls = removePassword(Config.Caddy.Api_Urls)
		fmt.Println("Caddy API URLs: " + strings.Join(CensoredApiUrls, ", "))
	}

	if Config.Caddy.Loop_Order != "SERVERS" && Config.Caddy.Loop_Order != "API_URLS" {
		log.Error().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "validation").Msg("Invalid loop order")
		os.Exit(1)
	}
	log.Debug().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "loop_order").Msg("Loop order is " + Config.Caddy.Loop_Order)

	// Her API URL (LB) bağımsız bir iştir: kendi Caddy'sine yazar, diğerlerini beklemez.
	// Varsayılan eşzamanlılık = API URL sayısı; `parallel_workers` ile sınırlanabilir.
	workers := len(Config.Caddy.Api_Urls)
	if Config.Caddy.Parallel_Workers > 0 && Config.Caddy.Parallel_Workers < workers {
		workers = Config.Caddy.Parallel_Workers
	}
	if workers < 1 {
		workers = 1
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		badUrls []string
	)

	tasks := make(chan string)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for apiUrl := range tasks {
				url := strings.Split(apiUrl, "@")[1]
				usernamePassword := strings.Split(apiUrl, "@")[0]
				idless := strings.Split(url, ";")[0]

				for _, urlToFind := range Config.Caddy.Servers {
					fmt.Println("Checking " + urlToFind + " on " + idless)
					if err := IdentifyRequest(server, url, usernamePassword, urlToFind); err != nil {
						fmt.Println("Failed to switch upstreams for " + idless + ": " + err.Error())
						mu.Lock()
						badUrls = append(badUrls, idless)
						mu.Unlock()
					}
				}

				// Rotalar tek tek PATCH edilmez (her PATCH tam bir config reload'u
				// demektir); LB başına toplanan değişiklikler tek /load ile uygulanır.
				if err := ApplyPendingChanges(idless, usernamePassword); err != nil {
					fmt.Println("Failed to switch upstreams for " + idless + ": " + err.Error())
					AlarmCustom("red_circle", "Failed to switch upstreams for "+idless+": "+strings.ReplaceAll(err.Error(), "\"", "'"))
					mu.Lock()
					badUrls = append(badUrls, idless)
					mu.Unlock()
				}

				if Config.Caddy.Lb_Policy_Change_Sleep > 0 {
					time.Sleep(Config.Caddy.Lb_Policy_Change_Sleep * time.Second)
				}
			}
		}()
	}
	for _, apiUrl := range Config.Caddy.Api_Urls {
		tasks <- apiUrl
	}
	close(tasks)
	wg.Wait()

	badUrls = uniqueSorted(badUrls)

	if Config.Caddy.Loop_Order == "SERVERS" {
		if len(badUrls) > 0 {
			badUrlsHumanReadable := strings.Join(badUrls, ", ")
			fmt.Println("Failed to switch upstreams for the following URLs: " + badUrlsHumanReadable)
			AlarmCustom("yellow_circle", "Partially failed to switch upstreams to "+server+" for the following servers: "+strings.Join(Config.Caddy.Servers, ", ")+". Failed to switch upstreams for the following URLs: "+badUrlsHumanReadable)
		} else {
			AlarmCustom("green_circle", "The URL(s) "+strings.Join(Config.Caddy.Servers, ", ")+" have been completely switched to "+server)
		}
	} else {
		if len(badUrls) > 0 {
			AlarmCustom("yellow_circle", "Partially failed to switch upstreams to "+server+" for the following API URLs: "+strings.Join(badUrls, ", "))
		} else {
			AlarmCustom("green_circle", "The URL(s) "+strings.Join(CensoredApiUrls, ", ")+" have been completely switched to "+server)
		}
	}
}

func ParseQuick[T int | map[string]interface{}](query string, json map[string]interface{}, server string, urlToFind string) (T, error) {
	var res T
	code, err := gojq.Parse(query)
	if err != nil {
		return res, err
	}
	compiled, err := gojq.Compile(
		code,
		gojq.WithVariables([]string{"$server", "$domain"}),
	)
	if err != nil {
		return res, err
	}
	iter := compiled.Run(json, server, urlToFind)
	for {
		result, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := result.(error); ok {
			return res, err
		}
		if v, ok := result.(T); ok {
			res = v
			return res, nil
		}
		return res, fmt.Errorf("unexpected jq result type")
	}
	return res, nil
}

func ParseChangeUpstreams(query string, json map[string]interface{}, variable []string) map[string]interface{} {
	var res map[string]interface{}
	code, err := gojq.Parse(query)
	if err != nil {
		return res
	}
	compiled, err := gojq.Compile(
		code,
		gojq.WithVariables([]string{variable[0]}),
	)
	if err != nil {
		return res
	}
	iter := compiled.Run(json, variable[1])
	for {
		result, ok := iter.Next()
		if !ok {
			break
		}
		if _, ok := result.(error); ok {
			return res
		}
		if m, ok := result.(map[string]interface{}); ok {
			res = m
			return res
		}
		return res
	}
	return res
}

func hasExactHost(route map[string]interface{}, domain string) bool {
	matchRaw, ok := route["match"]
	if !ok {
		return false
	}
	matches, ok := matchRaw.([]interface{})
	if !ok {
		return false
	}
	for _, m := range matches {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if hostsRaw, ok := mm["host"]; ok {
			if hosts, ok := hostsRaw.([]interface{}); ok {
				for _, hv := range hosts {
					if hs, ok := hv.(string); ok && hs == domain {
						return true
					}
				}
			}
		}
	}
	return false
}

func hasReverseProxy(route map[string]interface{}) bool {
	handlesRaw, ok := route["handle"]
	if !ok {
		return false
	}
	handles, ok := handlesRaw.([]interface{})
	if !ok {
		return false
	}

	for _, h := range handles {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		// düz reverse_proxy
		if handler, _ := hm["handler"].(string); handler == "reverse_proxy" {
			return true
		}
		// subroute → routes → handle → reverse_proxy
		if handler, _ := hm["handler"].(string); handler == "subroute" {
			if rsRaw, ok := hm["routes"]; ok {
				if rs, ok := rsRaw.([]interface{}); ok {
					for _, r := range rs {
						if rm, ok := r.(map[string]interface{}); ok {
							if hhRaw, ok := rm["handle"]; ok {
								if hh, ok := hhRaw.([]interface{}); ok {
									for _, hhx := range hh {
										if hhm, ok := hhx.(map[string]interface{}); ok {
											if hname, _ := hhm["handler"].(string); hname == "reverse_proxy" {
												return true
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

func pickRoute(respBodyJson map[string]interface{}, server string, domain string) (int, map[string]interface{}, error) {
	srvRaw, ok := respBodyJson[server]
	if !ok {
		return 0, nil, fmt.Errorf("server key %s not found", server)
	}
	srv, ok := srvRaw.(map[string]interface{})
	if !ok {
		return 0, nil, fmt.Errorf("server %s is not an object", server)
	}
	routesRaw, ok := srv["routes"]
	if !ok {
		return 0, nil, fmt.Errorf("server %s has no routes", server)
	}
	routes, ok := routesRaw.([]interface{})
	if !ok {
		return 0, nil, fmt.Errorf("server %s routes is not an array", server)
	}

	for i, r := range routes {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if hasExactHost(rm, domain) && hasReverseProxy(rm) {
			return i, rm, nil
		}
	}
	return 0, nil, fmt.Errorf("no route matched host=%s with reverse_proxy under server=%s", domain, server)
}

func IdentifyRequest(srvArg string, url string, usernamePassword string, urlToFind string) error {
	identifier := strings.Split(url, ";")[1]
	actualUrl := strings.Split(url, ";")[0]
	fmt.Println("Checking " + actualUrl + " for " + identifier)
	log.Debug().Str("component", "lbPolicy").Str("operation", "IdentifyRequest").Str("action", "get_servers").Msg("GET " + actualUrl + "/config/apps/http/servers")

	respBodyJson, err := fetchServersConfig(actualUrl, usernamePassword)
	if err != nil {
		return err
	}

	var servers []string
	for server := range respBodyJson {
		servers = append(servers, server)
	}
	sort.Strings(servers)

	fmt.Println("Servers: " + strings.Join(servers, ", "))

	for _, server := range servers {
		if _, ok := respBodyJson[server]; !ok {
			log.Debug().Str("component", "lbPolicy").Str("operation", "IdentifyRequest").Str("action", "missing_server_key").Msg("Server key not found: " + server)
			continue
		}

		fmt.Println("Checking " + server)

		routeId, routeObj, err := pickRoute(respBodyJson, server, urlToFind)
		if err != nil {
			log.Debug().Str("component", "lbPolicy").Str("operation", "IdentifyRequest").Str("action", "pick_route_failed").Msg(err.Error())
			continue
		}

		if dbg, e := json.Marshal(routeObj); e == nil {
			log.Debug().
				Str("component", "lbPolicy").
				Str("operation", "IdentifyRequest").
				Str("action", "matched_route").
				RawJSON("route", dbg).
				Msg("Matched route payload before transform")
		}

		ChangeUpstreams(urlToFind, srvArg, identifier, url, actualUrl, server, routeId, routeObj)
	}

	return nil
}

func ChangeUpstreams(urlToFind string, switchTo string, identifier string, url string, actualUrl string, server string, routeId int, req map[string]interface{}) {
	if n := atomic.LoadInt64(&noChangesCounter); n > int64(Config.Caddy.Nochange_Exit_Threshold) {
		fmt.Println("No changes were made for " + strconv.FormatInt(n, 10) + " times.")
		os.Exit(0)
	}

	reqUrl := actualUrl + "/config/apps/http/servers/" + server + "/routes/" + strconv.Itoa(routeId)
	fmt.Println("Changing " + reqUrl + " to " + identifier)

	if strings.Contains(switchTo, "first_") {
		second := strings.Split(switchTo, "_")[1]
		fmt.Println("Switching to " + second)
		log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "switching_to_first").Msg("Switching to " + second)
		log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "request_data").Msg("req: " + fmt.Sprintf("%v", req))

		reqToSend := ParseChangeUpstreams(`
		  .handle = (
		    (.handle // [])
		    | map(
		        if .handler == "subroute" and (.routes? != null) then
		          .routes = (
		            (.routes // [])
		            | map(
		                .handle = (
		                  (.handle // [])
		                  | map(
		                      if .handler == "reverse_proxy" then
		                        (
		                          if ((.upstreams // []) | length) == 2
		                             and ((.upstreams[1].dial // "") | contains($SRVNAME))
		                            then .upstreams |= [.[1], .[0]]
		                            else .
		                          end
		                        )
		                        | (.load_balancing.selection_policy.policy = "first")
		                      else . end
		                  )
		                )
		            )
		          )
		        elif .handler == "reverse_proxy" then
		          (
		            if ((.upstreams // []) | length) == 2
		               and ((.upstreams[1].dial // "") | contains($SRVNAME))
		              then .upstreams |= [.[1], .[0]]
		              else .
		            end
		          )
		          | (.load_balancing.selection_policy.policy = "first")
		        else . end
		    )
		  )
		`, req, []string{"$SRVNAME", second})

		log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "request_to_send").Msg("reqToSend: " + fmt.Sprintf("%v", reqToSend))

		if reqToSend == nil {
			log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "nil_payload").Msg("jq produced nil request payload")
			atomic.AddInt64(&noChangesCounter, 1)
			return
		}

		if reflect.DeepEqual(reqToSend, req) && !Config.Caddy.Override_Config {
			fmt.Println("No changes were made as the upstreams are already in " + second + " order")
			atomic.AddInt64(&noChangesCounter, 1)
			return
		}

		fmt.Println("Staging lb_policy change to " + switchTo + " for " + url)
		stageRouteChange(actualUrl, stagedRouteChange{
			server: server,
			index:  routeId,
			route:  reqToSend,
			line:   url + "'s upstream has been switched to " + switchTo,
		})

	} else if switchTo == "round_robin" || switchTo == "ip_hash" {
		log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "switching_policy").Msg("Switching to " + switchTo)
		log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "request_data").Msg("req: " + fmt.Sprintf("%v", req))

		reqToSend := ParseChangeUpstreams(`
		  .handle = (
		    (.handle // [])
		    | map(
		        if .handler == "subroute" and (.routes? != null) then
		          .routes = (
		            (.routes // [])
		            | map(
		                .handle = (
		                  (.handle // [])
		                  | map(
		                      if .handler == "reverse_proxy"
		                        then .load_balancing.selection_policy.policy = $LB_POLICY
		                        else .
		                      end
		                  )
		                )
		            )
		          )
		        elif .handler == "reverse_proxy"
		          then .load_balancing.selection_policy.policy = $LB_POLICY
		        else . end
		    )
		  )
		`, req, []string{"$LB_POLICY", switchTo})

		log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "request_to_send").Msg("reqToSend: " + fmt.Sprintf("%v", reqToSend))

		if reqToSend == nil {
			log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "nil_payload").Msg("jq produced nil request payload")
			atomic.AddInt64(&noChangesCounter, 1)
			return
		}

		if reflect.DeepEqual(reqToSend, req) && !Config.Caddy.Override_Config {
			fmt.Println("No changes were made as the upstreams are already in " + switchTo + " order")
			atomic.AddInt64(&noChangesCounter, 1)
			return
		}

		fmt.Println("Staging lb_policy change to " + switchTo + " for " + url)
		stageRouteChange(actualUrl, stagedRouteChange{
			server: server,
			index:  routeId,
			route:  reqToSend,
			line:   url + "'s upstream has been switched to " + switchTo,
		})
	} else {
		log.Error().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "validation").Msg("Invalid load balancing policy")
		os.Exit(1)
	}

	_ = os.MkdirAll("/tmp/glb/"+urlToFind+"/"+identifier, os.ModePerm)
	common.WriteToFile("/tmp/glb/"+urlToFind+"/"+identifier+"/lb_policy", switchTo)
}

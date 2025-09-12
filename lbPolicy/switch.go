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
	"time"

	"github.com/itchyny/gojq"
	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

var noChangesCounter int

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
	resp, err := http.Get(url)
	maxRetries := 2
	if err != nil {
		for i := 0; i < maxRetries; i++ {
			err = nil
			fmt.Println("Retrying " + url)
			resp, err = http.Get(url)
			if err == nil {
				break
			}
		}
		if err != nil {
			return "", err
		}
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
		for _, server := range Config.Caddy.Api_Urls {
			log.Debug().Str("component", "lbPolicy").Str("operation", "AdjustApiUrls").Str("action", "checking_server").Msg("Checking " + server + " under " + lbUrl)

			url := strings.Split(server, "@")[1]
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

	if Config.Caddy.Loop_Order == "SERVERS" {
		log.Debug().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "loop_order").Msg("Loop order is SERVERS")
		var badUrls []string
		for _, urlToFind := range Config.Caddy.Servers {
			for urlUp := range Config.Caddy.Api_Urls {
				url := strings.Split(Config.Caddy.Api_Urls[urlUp], "@")[1]
				usernamePassword := strings.Split(Config.Caddy.Api_Urls[urlUp], "@")[0]
				fmt.Println("Checking " + urlToFind + " on " + url)
				err := IdentifyRequest(server, url, usernamePassword, urlToFind)
				if err != nil {
					fmt.Println("Failed to switch upstreams for " + url + ": " + err.Error())
					badUrls = append(badUrls, url)
				}
			}
			time.Sleep(Config.Caddy.Lb_Policy_Change_Sleep * time.Second)
		}
		if len(badUrls) > 0 {
			badUrlsHumanReadable := strings.Join(badUrls, ", ")
			fmt.Println("Failed to switch upstreams for the following URLs: " + badUrlsHumanReadable)
			AlarmCustom("yellow_circle", "Partially failed to switch upstreams to "+server+" for the following servers: "+strings.Join(Config.Caddy.Servers, ", ")+". Failed to switch upstreams for the following URLs: "+badUrlsHumanReadable)
		} else {
			AlarmCustom("green_circle", "The URL(s) "+strings.Join(Config.Caddy.Servers, ", ")+" have been completely switched to "+server)
		}
	} else if Config.Caddy.Loop_Order == "API_URLS" {
		log.Debug().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "loop_order").Msg("Loop order is API_URLS")
		var badUrls []string
		for urlUp := range Config.Caddy.Api_Urls {
			log.Debug().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "checking_api_url").Msg("Checking " + Config.Caddy.Api_Urls[urlUp])
			for _, urlToFind := range Config.Caddy.Servers {
				log.Debug().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "checking_server").Msg("Checking " + urlToFind + " on " + Config.Caddy.Api_Urls[urlUp])
				url := strings.Split(Config.Caddy.Api_Urls[urlUp], "@")[1]
				usernamePassword := strings.Split(Config.Caddy.Api_Urls[urlUp], "@")[0]
				fmt.Println("Checking " + urlToFind + " on " + url)
				err := IdentifyRequest(server, url, usernamePassword, urlToFind)
				if err != nil {
					fmt.Println("Failed to switch upstreams for " + url + ": " + err.Error())
					badUrls = append(badUrls, url)
				}
			}
			time.Sleep(Config.Caddy.Lb_Policy_Change_Sleep * time.Second)
		}
		if len(badUrls) > 0 {
			AlarmCustom("yellow_circle", "Partially failed to switch upstreams to "+server+" for the following API URLs: "+strings.Join(badUrls, ", "))
		} else {
			AlarmCustom("green_circle", "The URL(s) "+strings.Join(CensoredApiUrls, ", ")+" have been completely switched to "+server)
		}
	} else {
		log.Error().Str("component", "lbPolicy").Str("operation", "SwitchMain").Str("action", "validation").Msg("Invalid loop order")
		os.Exit(1)
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

	req, err := http.NewRequest("GET", actualUrl+"/config/apps/http/servers", nil)
	if err != nil {
		log.Debug().Str("component", "lbPolicy").Str("operation", "IdentifyRequest").Str("action", "create_request").Msg("Failed to create request: " + err.Error())
		return err
	}
	req.SetBasicAuth(strings.Split(usernamePassword, ":")[0], strings.Split(usernamePassword, ":")[1])

	client := newHTTPClient()
	resp, err := doWithRetry(client, req, 5)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s returned %s; body=%s", actualUrl+"/config/apps/http/servers", resp.Status, string(b))
	}

	gojqQuery, err := gojq.Parse("keys[]")
	if err != nil {
		return err
	}

	var servers []string
	var respBodyJson map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respBodyJson); err != nil {
		return err
	}

	gojqQueryIter := gojqQuery.Run(respBodyJson)
	if gojqQueryIter == nil {
		return errors.New("gojqQueryIter is nil")
	}
	for {
		result, ok := gojqQueryIter.Next()
		if !ok {
			break
		}
		if s, ok := result.(string); ok {
			servers = append(servers, s)
		}
	}

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

		ChangeUpstreams(urlToFind, srvArg, identifier, url, actualUrl, server, routeId, routeObj, usernamePassword)
	}

	return nil
}

func SendRequest(jsonPayload map[string]interface{}, url string, usernamePassword string) error {
	payloadBytes, err := json.Marshal(jsonPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payloadBytes)), nil
	}
	req.ContentLength = int64(len(payloadBytes))

	if usernamePassword != "" {
		credentials := strings.SplitN(usernamePassword, ":", 2)
		if len(credentials) != 2 {
			return fmt.Errorf("invalid usernamePassword format, expected 'username:password'")
		}
		req.SetBasicAuth(credentials[0], credentials[1])
	}

	client := newHTTPClient()
	resp, err := doWithRetry(client, req, 5)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("received non-2xx response: %s; body=%s", resp.Status, string(b))
	}
	return nil
}

func ChangeUpstreams(urlToFind string, switchTo string, identifier string, url string, actualUrl string, server string, routeId int, req map[string]interface{}, UsernamePassword string) {
	if noChangesCounter > Config.Caddy.Nochange_Exit_Threshold {
		fmt.Println("No changes were made for " + strconv.Itoa(noChangesCounter) + " times.")
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
			noChangesCounter++
			return
		}

		if reflect.DeepEqual(reqToSend, req) && !Config.Caddy.Override_Config {
			fmt.Println("No changes were made as the upstreams are already in " + second + " order")
			noChangesCounter++
			return
		}

		fmt.Println("Sending request to change lb_policy to " + switchTo)
		err := SendRequest(reqToSend, reqUrl, UsernamePassword)
		if err == nil {
			fmt.Println(url + "'s upstream has been switched to " + switchTo)
		} else {
			fmt.Println("Failed to switch " + url + "'s upstream to " + switchTo)
			log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "send_request_error").Msg(err.Error())
			AlarmCustom("red_circle", "Failed to switch "+url+"'s upstream to "+switchTo+": "+strings.ReplaceAll(err.Error(), "\"", "'"))
		}

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
			noChangesCounter++
			return
		}

		if reflect.DeepEqual(reqToSend, req) && !Config.Caddy.Override_Config {
			fmt.Println("No changes were made as the upstreams are already in " + switchTo + " order")
			noChangesCounter++
			return
		}

		fmt.Println("Sending request to change lb_policy to " + switchTo)
		err := SendRequest(reqToSend, reqUrl, UsernamePassword)
		if err == nil {
			fmt.Println(url + "'s upstream has been switched to " + switchTo)
		} else {
			fmt.Println("Failed to switch " + url + "'s upstream to " + switchTo)
			log.Debug().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "send_request_error").Msg(err.Error())
			AlarmCustom("red_circle", "Failed to switch "+url+"'s upstream to "+switchTo+": "+strings.ReplaceAll(err.Error(), "\"", "'"))
		}
	} else {
		log.Error().Str("component", "lbPolicy").Str("operation", "ChangeUpstreams").Str("action", "validation").Msg("Invalid load balancing policy")
		os.Exit(1)
	}

	_ = os.MkdirAll("/tmp/glb/"+urlToFind+"/"+identifier, os.ModePerm)
	common.WriteToFile("/tmp/glb/"+urlToFind+"/"+identifier+"/lb_policy", switchTo)
}

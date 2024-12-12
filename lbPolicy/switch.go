package lbPolicy

import (
    "os"
    "fmt"
    "sort"
    "time"
    "bufio"
    "bytes"
    "errors"
    "strconv"
    "reflect"
    "strings"
    "net/http"
    "encoding/json"
    "github.com/itchyny/gojq"
	"github.com/monobilisim/monokit/common"
)

var noChangesCounter int

func hostnameToURL(hostname string) (string, error) {
	// Split the hostname into parts
	parts := strings.Split(hostname, "-")

	// Check if we have enough parts
	if len(parts) < 3 {
		return "", errors.New("invalid hostname format")
	}

	// Extract the relevant parts
	domainPart := parts[0]
	envPart := parts[1]
	lbPart := parts[2]

	// Construct the URL
	url := fmt.Sprintf("https://api.%s.%s.%s.biz.tr", lbPart, envPart, domainPart)
	return url, nil
}

func extractHostname(url string) (string, error) {
    
    resp, err := http.Get(url)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	// Scan the response body line by line
	scanner := bufio.NewScanner(resp.Body)
	var hostname string
	for scanner.Scan() {
		line := scanner.Text()
		// Check if the line contains "Hostname:"
		if strings.Contains(line, "Hostname:") {
			// Split the line and get the second part (after the space)
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

	// Print the extracted hostname
	if hostname != "" {
        return hostname, nil
	} else {
	    return "", errors.New("hostname not found")
    }
}

func removePassword(urls []string) []string {
	var censoredCaddyAPIUrls []string

	for _, url := range urls {
		// Find the part of the URL after the semicolon.
		if idx := strings.Index(url, ";"); idx != -1 {
			censoredCaddyAPIUrls = append(censoredCaddyAPIUrls, url[idx+1:])
		} else {
			// If no semicolon is found, add the original URL.
			censoredCaddyAPIUrls = append(censoredCaddyAPIUrls, url)
		}
	}

	return censoredCaddyAPIUrls
}

func uniqueSorted(input []string) []string {
	// Sort the array
	sort.Strings(input)

	// Remove duplicates
	var unique []string
	for i, val := range input {
		// Add the value if it's the first element or different from the previous one
		if i == 0 || val != input[i-1] {
			unique = append(unique, val)
		}
	}

	return unique
}

func AdjustApiUrls() {
    var caddyApiUrlsNew []string

    for _, lbUrl := range Config.Caddy.Lb_Urls {
        for _, server := range Config.Caddy.Api_Urls {
            if len(caddyApiUrlsNew) == len(Config.Caddy.Lb_Urls)-1 {
                break
            }

            url := strings.Split(server, "@")[1]
            //usernamePassword := strings.Split(server, "@")[0]
            
            hostname, err := extractHostname(lbUrl)

            if err != nil {
                common.LogError(err.Error())
                continue
            }

            urlNew, err := hostnameToURL(hostname)

            if err != nil {
                common.LogError(err.Error())
                continue
            }

            if urlNew == url {
                fmt.Println(urlNew + " is the same as URL, adding to caddyApiUrlsNew")
                caddyApiUrlsNew = append(caddyApiUrlsNew, server) // Make sure the ones that respond first are added first
            }
        }
    }

    for _, urlUp := range Config.Caddy.Api_Urls {
        caddyApiUrlsNew = append(caddyApiUrlsNew, urlUp)
    }
    
    // Remove duplicates
    Config.Caddy.Api_Urls = uniqueSorted(caddyApiUrlsNew)
}


func SwitchMain(server string) {
    var CensoredApiUrls []string

    if Config.Caddy.Loop_Order == "" {
        Config.Caddy.Loop_Order = "API_URLS"
    }

    if Config.Caddy.Api_Urls == nil || len(Config.Caddy.Api_Urls) == 0 {
        common.LogError("Api_Urls is not defined in caddy config")
        os.Exit(1)
    }

    if Config.Caddy.Servers == nil || len(Config.Caddy.Servers) == 0 {
        common.LogError("Servers is not defined in caddy config")
        os.Exit(1)
    }

    if (Config.Caddy.Lb_Urls == nil || len(Config.Caddy.Lb_Urls) == 0) && Config.Caddy.Dynamic_Api_Urls {
        common.LogError("Lb_Urls is not defined in caddy config, but Dynamic_Api_Urls is enabled")
        os.Exit(1)
    }

    if Config.Caddy.Dynamic_Api_Urls {
        AdjustApiUrls()
        CensoredApiUrls = removePassword(Config.Caddy.Api_Urls)
        fmt.Println("Caddy API URLs: " + strings.Join(CensoredApiUrls, ", "))
    }

    if Config.Caddy.Loop_Order == "SERVERS" {
        //var caddyServersWithoutBadUrls []string
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
            common.Alarm("[lbPolicy - " + common.Config.Identifier + "] [:yellow_circle:] Partially failed to switch upstreams for the following servers: " + strings.Join(Config.Caddy.Servers, ", ") + ". Failed to switch upstreams for the following URLs: " + badUrlsHumanReadable, "", "", false)
        } else {
            common.Alarm("[lbPolicy - " + common.Config.Identifier + "] [:green_circle:] The URL(s) " + strings.Join(Config.Caddy.Servers, ", ") + " have been completely switched to " + server, "", "", false)
        }

    } else if Config.Caddy.Loop_Order == "API_URLS" {
        for urlUp := range Config.Caddy.Api_Urls {
            for _, urlToFind := range Config.Caddy.Servers {
                url := strings.Split(Config.Caddy.Api_Urls[urlUp], "@")[1]
                usernamePassword := strings.Split(Config.Caddy.Api_Urls[urlUp], "@")[0]
                fmt.Println("Checking " + urlToFind + " on " + url)
                err := IdentifyRequest(server, url, usernamePassword, urlToFind)
                if err != nil {
                    fmt.Println("Failed to switch upstreams for " + url + ": " + err.Error())
                }
            }
            time.Sleep(Config.Caddy.Lb_Policy_Change_Sleep * time.Second)
        }
        common.Alarm("[lbPolicy - " + common.Config.Identifier + "] [:green_circle:] The URL(s) " + strings.Join(CensoredApiUrls, ", ") + " have been completely switched to " + server, "", "", false)
    } else {
        common.LogError("Invalid loop order")
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
        gojq.WithVariables([]string{
            "$server", "$domain",
        }),
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
        res = result.(T)
        return res, nil
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
        res = result.(map[string]interface{})
        return res
    }

    return res
}


func IdentifyRequest(srvArg string, url string, usernamePassword string, urlToFind string) (error) {
    identifier := strings.Split(url, ";")[1]
    actualUrl := strings.Split(url, ";")[0]
    fmt.Println("Checking " + actualUrl + " for " + identifier)

    req, err := http.NewRequest("GET", actualUrl + "/config/apps/http/servers", nil)

    if err != nil {
        return err
    }

    req.SetBasicAuth(strings.Split(usernamePassword, ":")[0], strings.Split(usernamePassword, ":")[1])
    client := &http.Client{Timeout: time.Second * 10}
   
    maxRetries := 2
    resp, err := client.Do(req)

    
    if err != nil {
        for i := 0; i < maxRetries; i++ {
            fmt.Println("Retrying " + actualUrl + " for " + identifier)
            err = nil
            resp, err = client.Do(req)
            if err == nil {
                break
            }
        }

        if err != nil {
            return err
        }
    }

    defer resp.Body.Close()
    
    gojqQuery, err := gojq.Parse("keys | join(\" \")")

    if err != nil {
        return  err
    }

    var servers []string

    var respBodyJson map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&respBodyJson)
    
    gojqQueryIter := gojqQuery.Run(respBodyJson)

    if gojqQueryIter == nil { 
        return errors.New("gojqQueryIter is nil")
    }

    for {
		result, ok := gojqQueryIter.Next()
		if !ok {
			break // No more results
		}
		
	    servers = append(servers, result.(string))
    }

    fmt.Println("Servers: " + strings.Join(servers, ", "))

    for _, server := range servers {
        fmt.Println("Checking " + server)
        var routeId int
        request, err := ParseQuick[map[string]interface{}](`
            .[$server].routes[]
                | select(
                    (.match[] | (.host | index($domain)) != null)
                    and
                    (.handle[].routes[].handle[].upstreams != null)
                )`, respBodyJson, server, urlToFind)

        if err != nil {
            return err
        }

        routeId, err = ParseQuick[int](`
            .[$server].routes
                | to_entries[]
                | select(
                    (.value.match[] | (.host | index($domain)) != null)
                    and
                    (.value.handle[].routes[].handle[].upstreams != null)
                )
                | .key
        `, respBodyJson, server, urlToFind)

        if err != nil {
            return err
        }

        if request != nil {
            ChangeUpstreams(srvArg, identifier, url, actualUrl, server, routeId, request, usernamePassword)
        }

    }

    return nil
}

func SendRequest(jsonPayload map[string]interface{}, url string, usernamePassword string) error {
	// Convert the JSON payload to a byte array
	payloadBytes, err := json.Marshal(jsonPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set the Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// Add Basic Auth if username and password are provided
	if usernamePassword != "" {
		credentials := strings.SplitN(usernamePassword, ":", 2)
		if len(credentials) != 2 {
			return fmt.Errorf("invalid usernamePassword format, expected 'username:password'")
		}
		req.SetBasicAuth(credentials[0], credentials[1])
	}

	// Send the request using the HTTP client
    client := &http.Client{Timeout: time.Second * 10}
    maxRetries := 2
	resp, err := client.Do(req)

	if err != nil {
        for i := 0; i < maxRetries; i++ {
            fmt.Println("Retrying " + url)
            err = nil
            resp, err = client.Do(req)
            if err == nil {
                break
            }
        }

        if err != nil {
		    return fmt.Errorf("failed to send HTTP request: %w", err)
        }
	}
	defer resp.Body.Close()

	// Check the HTTP response status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received non-2xx response: %s", resp.Status)
	}

	return nil
}

func ChangeUpstreams(switchTo string, identifier string, url string, actualUrl string, server string, routeId int, req map[string]interface{}, UsernamePassword string) {
    if noChangesCounter > Config.Caddy.Nochange_Exit_Threshold {
        fmt.Println("No changes were made for " + strconv.Itoa(noChangesCounter) + " times.")
        os.Exit(0)
    }

    reqUrl := actualUrl + "/config/apps/http/servers/" + server + "/routes/" + strconv.Itoa(routeId)

    fmt.Println("Changing " + reqUrl + " to " + identifier)

    switch switchTo {
        case "first_dc1", "first_dc2":
            second := strings.Split(switchTo, "_")[1]
            fmt.Println("Switching to " + second)
            reqToSend := ParseChangeUpstreams(`
                .handle[] |= (
                  .routes[] |= (
                    .handle[] |= (
                      if .handler == "reverse_proxy" then
                        (
                          if (.upstreams | length) == 2 and (.upstreams[1].dial | contains($SRVNAME))
                            then .upstreams |= [.[1], .[0]]
                            else .
                          end
                        )
                        | (.load_balancing.selection_policy.policy = "first") # Set policy here
                      else .
                      end
                    )
                  )
                )`, req, []string{"$SRVNAME", second})
                
                if reflect.DeepEqual(reqToSend, req) && !Config.Caddy.Override_Config {
                    fmt.Println("No changes were made as the upstreams are already in " + second + " order")
                    noChangesCounter++
                    return
                }

                fmt.Println("Sending request to change lb_policy to " + switchTo)

                // Send the request
                err := SendRequest(reqToSend, reqUrl, UsernamePassword)

                if err == nil {
                    fmt.Println(url + "'s upstream has been switched to " + switchTo)
                } else {
                    fmt.Println("Failed to switch " + url + "'s upstream to " + switchTo)
                    common.AlarmCheckDown("failupstrm_" + switchTo, "Failed to switch " + url + "'s upstream to " + switchTo + ": " + strings.ReplaceAll(err.Error(), "\"", "'"), true)
                }

        case "round_robin", "ip_hash":
            reqToSend := ParseChangeUpstreams(`
                .handle[] |= (
                  .routes[] |= (
                    .handle[] |= (
                      if .handler == "reverse_proxy"
                      then .load_balancing.selection_policy.policy = $LB_POLICY
                      else .
                      end
                    )
                  )
                )`, req, []string{"$LB_POLICY", switchTo})

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
                common.AlarmCheckDown("failupstrm_" + switchTo, "Failed to switch " + url + "'s upstream to " + switchTo + ": " + err.Error(), true)
            }

        default:
            common.LogError("Invalid load balancing policy")
            os.Exit(1)
    }

    os.MkdirAll("/tmp/glb/" + actualUrl + "/" + identifier, os.ModePerm)
    common.WriteToFile("/tmp/glb/" + actualUrl + "/" + identifier + "/lb_policy", switchTo)
}

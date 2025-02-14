package common

import (
	"net/http"
)

// AddUserAgent adds the Monokit user agent to the request
func AddUserAgent(req *http.Request) {
	req.Header.Set("User-Agent", "Monokit/"+MonokitVersion)
}

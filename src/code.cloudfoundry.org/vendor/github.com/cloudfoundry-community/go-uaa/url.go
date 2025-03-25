package uaa

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// BuildTargetURL returns a URL. If the target does not include a scheme, https
// will be used.
func BuildTargetURL(target string) (*url.URL, error) {
	if !strings.Contains(target, "://") {
		target = fmt.Sprintf("https://%s", target)
	}

	return url.Parse(target)
}

// BuildSubdomainURL returns a URL that optionally includes the zone ID as a host
// prefix. If the target does not include a scheme, https will be used.
func BuildSubdomainURL(target string, zoneID string) (*url.URL, error) {
	url, err := BuildTargetURL(target)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(url.Hostname(), zoneID) {
		url.Host = fmt.Sprintf("%s.%s", zoneID, url.Host)
	}

	return url, nil
}

// urlWithPath copies the URL and sets the path on the copy.
func urlWithPath(u url.URL, p string) url.URL {
	u.Path = path.Join(u.Path, p)
	return u
}

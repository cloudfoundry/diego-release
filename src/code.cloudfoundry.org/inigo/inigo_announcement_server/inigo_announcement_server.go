package inigo_announcement_server

import (
	"encoding/json"
	"fmt"
	"sync"

	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/inigo/helpers"
	. "github.com/onsi/gomega"
)

var server *httptest.Server
var serverAddr string

func Start(externalAddress string) {
	lock := &sync.RWMutex{}

	registered := []string{}

	server, serverAddr = helpers.Callback(externalAddress, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/announce":
			lock.Lock()
			registered = append(registered, r.URL.Query().Get("announcement"))
			lock.Unlock()
		case "/announcements":
			lock.RLock()
			// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
			json.NewEncoder(w).Encode(registered)
			lock.RUnlock()
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func Stop() {
	server.Close()
}

func AnnounceURL(announcement string) string {
	return fmt.Sprintf("http://%s/announce?announcement=%s", serverAddr, announcement)
}

func Announcements() []string {
	response, err := http.Get(fmt.Sprintf("http://%s/announcements", serverAddr))
	Expect(err).NotTo(HaveOccurred())

	defer response.Body.Close()

	var responses []string

	err = json.NewDecoder(response.Body).Decode(&responses)
	Expect(err).NotTo(HaveOccurred())

	return responses
}

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

var (
	memoryAllocated = flag.Uint("allocate-memory-b", 0, "allocate this much memory (in mb) on the heap and do not release it")
	//lint:ignore U1000 - we want this to allocate memory that we never release when configured as such
	someGarbage []uint8
)

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/env", env)
	http.HandleFunc("/write", write)
	http.HandleFunc("/curl", curl)
	http.HandleFunc("/yo", yo)
	http.HandleFunc("/privileged", privileged)
	http.HandleFunc("/cf-instance-cert", cfInstanceCert)
	http.HandleFunc("/cf-instance-key", cfInstanceKey)
	http.HandleFunc("/cat", catFile)

	if memoryAllocated != nil {
		someGarbage = make([]uint8, *memoryAllocated*1024*1024)
	}

	fmt.Println("listening...")

	ports := os.Getenv("PORT")
	httpsPort := os.Getenv("HTTPS_PORT")

	portArray := strings.Split(ports, " ")

	errCh := make(chan error)

	for _, port := range portArray {
		addr := ""
		if os.Getenv("SKIP_LOCALHOST_LISTEN") != "" {
			addr = os.Getenv("CF_INSTANCE_INTERNAL_IP")
		}
		addr += ":" + port
		go func(addr string) {
			server := &http.Server{
				Addr:              addr,
				Handler:           nil,
				ReadHeaderTimeout: 5 * time.Second,
			}
			errCh <- server.ListenAndServe()
		}(addr)
	}

	if httpsPort != "" {
		go func() {
			instanceCertPath := os.Getenv("CF_INSTANCE_CERT")
			instanceKeyPath := os.Getenv("CF_INSTANCE_KEY")
			server := &http.Server{
				Addr:              fmt.Sprintf(":%s", httpsPort),
				Handler:           nil,
				ReadHeaderTimeout: 5 * time.Second,
			}
			errCh <- server.ListenAndServeTLS(instanceCertPath, instanceKeyPath)
		}()
	}

	err := <-errCh
	if err != nil {
		panic(err)
	}
}

func hello(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(res, "%s", os.Getenv("INSTANCE_INDEX"))
}

func write(res http.ResponseWriter, req *http.Request) {
	mountPointPath := os.Getenv("MOUNT_POINT_DIR") + "/test.txt"

	d1 := []byte("Hello Persistent World!\n")
	err := os.WriteFile(mountPointPath, d1, 0644)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
		res.Write([]byte(err.Error()))
		return
	}

	res.WriteHeader(http.StatusOK)
	body, err := os.ReadFile(mountPointPath)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
		res.Write([]byte(err.Error()))
		return
	}
	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	res.Write(body)
}

func env(res http.ResponseWriter, req *http.Request) {
	for _, e := range os.Environ() {
		fmt.Fprintf(res, "%s\n", e)
	}
}

func curl(res http.ResponseWriter, req *http.Request) {
	cmd := exec.Command("curl", "--connect-timeout", "5", "http://www.example.com")
	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			fmt.Fprint(res, "Unknown Exit Code\n")
		}

		waitStatus := exitErr.Sys().(syscall.WaitStatus)
		fmt.Fprintf(res, "%d", waitStatus.ExitStatus())
		return
	}

	fmt.Fprintf(res, "%d", 0)
}

func yo(res http.ResponseWriter, req *http.Request) {
	fmt.Fprint(res, "sup dawg")
}

func privileged(res http.ResponseWriter, req *http.Request) {
	cmd := exec.Command("touch", "/proc/sysrq-trigger")
	err := cmd.Run()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, "Failed to touch file: %s\n", err.Error())
		return
	}

	fmt.Fprint(res, "Success\n")
}

func cfInstanceCert(res http.ResponseWriter, req *http.Request) {
	path := os.Getenv("CF_INSTANCE_CERT")

	data, err := os.ReadFile(path)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	res.Write(data)
}

func cfInstanceKey(res http.ResponseWriter, req *http.Request) {
	path := os.Getenv("CF_INSTANCE_KEY")

	data, err := os.ReadFile(path)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	res.Write(data)
}

func catFile(res http.ResponseWriter, req *http.Request) {
	param := req.URL.Query()
	fileToCat := param["file"][0]

	data, err := os.ReadFile(fileToCat)
	if os.IsNotExist(err) {
		res.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	res.Write(data)
}

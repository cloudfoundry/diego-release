package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type client struct {
	http *http.Client
}

func newClient() *client {
	ca, err := os.ReadFile("/etc/ssl/certs/rep/ca.crt")
	if err != nil {
		log.Fatalf("Failed to read CA certificate: %v", err)
	}

	cert, err := tls.LoadX509KeyPair("/etc/ssl/certs/rep/tls.crt", "/etc/ssl/certs/rep/tls.key")
	if err != nil {
		log.Fatalf("Failed to load client certificate/key: %v", err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(ca); !ok {
		logger.Fatal("Failed to append CA certificate")
	}

	c := &http.Client{
		Timeout: 30 * time.Second, // Set a timeout for the HTTP request
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      pool,
				Certificates: []tls.Certificate{cert},
			},
		},
	}

	return &client{
		http: c,
	}
}

var logger *log.Logger = log.Default()

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	nodeName := os.Getenv("NODE_NAME")
	log.Printf("Started on node %q\n", nodeName)

	c := newClient()

	lw := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"nodes",
		metav1.NamespaceAll,
		fields.OneTermEqualSelector("metadata.name", nodeName),
	)

	nodeInformer := cache.NewSharedInformer(
		lw,
		&v1.Node{},
		time.Second*10,
	)
	stopCh := make(chan struct{})

	pingPath := ""
	registration, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj any) {
			node := newObj.(*v1.Node)
			if node.Spec.Unschedulable {
				logger.Printf("Node %q is unschedulable, evacuating...\n", node.Name)
				pingPath = c.evacuate()
				stopCh <- struct{}{}
				return
			}
		},
	})
	if err != nil {
		panic(err)
	}

	go nodeInformer.Run(stopCh)

	<-stopCh
	err = nodeInformer.RemoveEventHandler(registration)
	if err != nil {
		panic(err)
	}

	c.pingForever(pingPath)
}

func (c *client) evacuate() string {
	resp, err := c.http.Do(&http.Request{
		Method: "POST",
		URL:    &url.URL{Scheme: "https", Host: "localhost:1800", Path: "/evacuate"},
	})
	if err != nil {
		logger.Printf("HTTP POST error: %v\n", err)
		return ""
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	logger.Printf("Evacuation response status: %s\n", resp.Status)
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Printf("Failed to read response body: %v\n", err)
			return ""
		}
		resultbody := map[string]string{}
		err = json.Unmarshal(body, &resultbody)
		if err != nil {
			logger.Printf("Failed to read unmarshal body: %v\n", err)
			return ""
		}
		return resultbody["ping_path"]
	} else {
		return ""
	}
}

func (c *client) pingForever(pingPath string) {
	for {
		resp, err := c.http.Do(&http.Request{
			Method: "GET",
			URL:    &url.URL{Scheme: "https", Host: "localhost:1800", Path: fmt.Sprintf("/%s", pingPath)},
		})
		if err != nil {
			logger.Printf("HTTP GET error: %v\n", err)
			return
		}
		logger.Printf("Ping response status: %s\n", resp.Status)
		time.Sleep(10 * time.Second) // Sleep for 10 seconds before next ping
	}
}

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	endpoints := os.Getenv("PROTOACTOR_ETCD_ENDPOINTS")
	if endpoints == "" {
		endpoints = "127.0.0.1:2379"
	}

	got, err := runGrainOnewayDemo(DemoConfig{
		EtcdEndpoints: strings.Split(endpoints, ","),
		BaseKey:       demoBaseKey("manual"),
		ClusterName:   "grain-oneway-demo",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "grain oneway demo failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("grain oneway demo succeeded, route key=%d\n", got)
}

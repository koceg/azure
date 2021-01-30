package main

// 20200130 get azure subscription Usage + quotas for region
// and expose the data as Prometheus HTTP metrics endpoint
import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	metrics := make(map[string]prometheus.Gauge)
	var region string
	var ok bool
	if region, ok = os.LookupEnv("AZURE_REGION"); !ok {
		log.Fatalf("Environment variable AZURE_REGION is missing")
	}
	ctx := context.Background()
	env, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		log.Fatalf("Environment Settings: %v", err)
	}
	subID := env.GetSubscriptionID()
	client := compute.NewUsageClient(subID)
	if client.Authorizer, err = env.GetAuthorizer(); err != nil {
		log.Fatalf("Authorisation fail: %v", err)
	}
	go func() {
		for {
			usage, err := client.List(ctx, region)
			if err != nil {
				log.Fatalf("Usage Limits: %v", err)
			}
			// every quota value is a separate Gauge
			for _, q := range usage.Values() {
				//fmt.Println(*q.Name.Value, *q.Name.LocalizedValue, *q.Unit, *q.CurrentValue, *q.Limit)
				n := strings.ToLower(*q.Name.Value)
				if _, ok := metrics[n]; !ok {
					metrics[n] = prometheus.NewGauge(prometheus.GaugeOpts{
						Namespace: "azure",
						Subsystem: region,
						Name:      strings.ReplaceAll(n, " ", "_"),
						Help:      fmt.Sprintf("%v Limit: %v", *q.Name.LocalizedValue, *q.Limit),
					})
					prometheus.MustRegister(metrics[n])
				}
				metrics[n].Set(float64(*q.CurrentValue))
			}
			time.Sleep(15 * time.Minute)
			//fmt.Println("------------------------------------------")
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}

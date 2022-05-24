package main

import (
	"github.com/itzg/go-flagsfiller"
	"github.com/itzg/zapconfigs"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned"
	"log"
	"os"
	"time"
)

const (
	DefaultInterval = 1 * time.Minute
)

var config struct {
	Namespace     string        `default:"default" usage:"the namespace of the pods to collect"`
	Interval      time.Duration `usage:"the interval of metrics collection"`
	IncludeLabels bool          `usage:"include pod labels in reported metrics"`
	Debug         bool          `usage:"enable debug logging"`
	Telegraf      struct {
		Endpoint string `usage:"if configured, metrics will be sent as line protocol to telegraf"`
	}
}

func main() {

	err := flagsfiller.Parse(&config, flagsfiller.WithEnv(""))
	if err != nil {
		log.Fatal(err)
	}

	var logger *zap.SugaredLogger
	if config.Debug {
		logger = zapconfigs.NewDebugLogger().Sugar()
	} else {
		logger = zapconfigs.NewDefaultLogger().Sugar()
	}
	defer logger.Sync()

	// Connect to kubernetes and get metrics clientset

	configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		logger.Fatalw("loading kubeConfig", "err", err)
	}

	var labelResolver LabelResolver
	if config.IncludeLabels {
		labelResolver, err = NewWatchingLabelResolver(kubeConfig, config.Namespace, logger)
		if err != nil {
			logger.Fatalw("creating kube clientset", "err", err)
		}
	} else {
		labelResolver = &DisabledLabelResolver{}
	}

	clientset, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		logger.Fatalw("creating metrics clientset", "err", err)
	}

	podMetricsAccessor := clientset.MetricsV1beta1().PodMetricses(config.Namespace)

	// Determine reporters

	var reporters []Reporter

	if config.Telegraf.Endpoint != "" {
		reporter, err := NewTelegrafReporter(config.Telegraf.Endpoint, logger)
		if err != nil {
			logger.Fatalw("creating telegraf reporter", "err", err)
		}
		reporters = append(reporters, reporter)
		if config.Interval == 0 {
			config.Interval = DefaultInterval
		}

		logger.Infow("reporting metrics to telegraf",
			"endpoint", config.Telegraf.Endpoint,
			"interval", config.Interval)
	}

	if len(reporters) == 0 {
		reporters = append(reporters, &StdoutReporter{})
	}

	if config.Interval > 0 {
		for {
			err = collect(podMetricsAccessor, reporters, labelResolver, config.Namespace)
			if err != nil {
				logger.Error("err", err)
				// go ahead and exit since there's probably a misconfig with metrics server, roles, etc
				os.Exit(1)
			}
			time.Sleep(config.Interval)
		}
	} else {
		err = collect(podMetricsAccessor, reporters, labelResolver, config.Namespace)
		if err != nil {
			logger.Error("err", err)
		}
	}
}

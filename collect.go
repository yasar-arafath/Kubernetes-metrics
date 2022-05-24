package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

func collect(podMetricsAccessor v1beta1.PodMetricsInterface, reporters []Reporter,
	labelResolver LabelResolver, namespace string) error {
	podMetricsList, err := podMetricsAccessor.List(v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list kube metrics: %w", err)
	}

	batches := make([]Batch, len(reporters))
	for i, reporter := range reporters {
		batches[i] = reporter.Start(namespace)
	}

	for _, p := range podMetricsList.Items {
		podName := p.Name
		labels := labelResolver.Resolve(podName)
		for _, c := range p.Containers {
			containerName := c.Name
			// matching the units reported by kubectl top pods
			cpuUsage := c.Usage.Cpu().ScaledValue(resource.Milli)
			memUsage := c.Usage.Memory().ScaledValue(resource.Mega)
			for _, batch := range batches {
				batch.Report(podName, containerName, labels, cpuUsage, memUsage)
			}
		}
	}

	for _, batch := range batches {
		_ = batch.Close()
	}

	return nil
}

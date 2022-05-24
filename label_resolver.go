package main

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"
)

type LabelResolver interface {
	Resolve(podName string) map[string]string
}

type WatchingLabelResolver struct {
	logger    *zap.SugaredLogger
	clientset *kubernetes.Clientset
	namespace string

	// labels maps pod name to labels
	labels     map[string]map[string]string
	labelsLock sync.RWMutex
}

func NewWatchingLabelResolver(c *rest.Config, namespace string, logger *zap.SugaredLogger) (*WatchingLabelResolver, error) {
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	w := &WatchingLabelResolver{
		clientset: clientset,
		logger:    logger.Named("label_resolver"),
		labels:    make(map[string]map[string]string),
		namespace: namespace,
	}
	go w.watch()

	// allow for initial pod-label loading
	time.Sleep(1 * time.Second)

	return w, nil
}

func (w *WatchingLabelResolver) Resolve(podName string) map[string]string {
	w.labelsLock.RLock()
	defer w.labelsLock.RUnlock()
	return w.labels[podName]
}

func (w *WatchingLabelResolver) watch() {
	listWatch := cache.NewListWatchFromClient(
		w.clientset.CoreV1().RESTClient(),
		string(corev1.ResourcePods),
		w.namespace,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		listWatch,
		&corev1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    w.add,
			UpdateFunc: w.update,
			DeleteFunc: w.delete,
		})

	w.logger.Infow("watching for pod label changes")
	controller.Run(nil)
}

func (w *WatchingLabelResolver) addUpdate(obj interface{}) {
	if pod, ok := obj.(*corev1.Pod); ok {
		w.labelsLock.Lock()
		w.labels[pod.Name] = pod.Labels
		w.labelsLock.Unlock()
	}
}

func (w *WatchingLabelResolver) add(obj interface{}) {
	w.addUpdate(obj)
}

func (w *WatchingLabelResolver) update(_ interface{}, newObj interface{}) {
	w.addUpdate(newObj)
}

func (w *WatchingLabelResolver) delete(obj interface{}) {
	if pod, ok := obj.(*corev1.Pod); ok {
		w.labelsLock.Lock()
		delete(w.labels, pod.Name)
		w.labelsLock.Unlock()
	}
}

type DisabledLabelResolver struct{}

func (d *DisabledLabelResolver) Resolve(string) map[string]string {
	return nil
}

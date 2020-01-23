package provider

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/provider"

	wave "github.com/marineghandevosyan/wavefront-kubernetes-adapter/pkg/client"
)

type MetricsLister interface {
	Run()
	RunUntil(stopChan <-chan struct{})
	ListCustomMetrics() []provider.CustomMetricInfo
	ListExternalMetrics() []provider.ExternalMetricInfo
}

type ExternalConfigListener interface {
	configChanged()
}

type WavefrontMetricsLister struct {
	Prefix          string
	UpdateInterval  time.Duration
	waveClient      wave.WavefrontClient
	externalDriver  ExternalMetricsDriver
	customMetrics   []provider.CustomMetricInfo
	externalMetrics []provider.ExternalMetricInfo
	lock            sync.RWMutex

	Translator
}

func (l *WavefrontMetricsLister) configChanged() {
	glog.V(5).Info("configuration changed. updating metrics.")
	l.updateMetrics()
}

func (l *WavefrontMetricsLister) Run() {
	l.RunUntil(wait.NeverStop)
}

func (l *WavefrontMetricsLister) RunUntil(stopChan <-chan struct{}) {
	// register with external driver for config changes
	l.externalDriver.registerListener(l)

	go wait.Until(func() {
		if err := l.updateMetrics(); err != nil {
			glog.Errorf("error updating metrics: %v", err)
		}
	}, l.UpdateInterval, stopChan)
}

func (l *WavefrontMetricsLister) updateMetrics() error {
	l.lock.Lock()
	defer l.lock.Unlock()
	customErr := l.updateCustomMetrics()
	externalErr := l.updateExternalMetrics()

	if customErr != nil || externalErr != nil {
		return fmt.Errorf("customMetricsError: %s, externalMetricsError: %s", customErr, externalErr)
	}
	return nil
}

func (l *WavefrontMetricsLister) updateCustomMetrics() error {
	metrics, err := l.waveClient.ListMetrics(l.Prefix + ".*")
	if err != nil {
		glog.Errorf("error retrieving list of custom metrics from Wavefront: %v", err)
		l.customMetrics = []provider.CustomMetricInfo{}
		return err
	}
	l.customMetrics = l.CustomMetricsFor(metrics)
	return nil
}

func (l *WavefrontMetricsLister) updateExternalMetrics() error {
	if l.externalDriver != nil {
		l.externalMetrics = l.ExternalMetricsFor(l.externalDriver.getMetricNames())
	}
	return nil
}

func (l *WavefrontMetricsLister) ListCustomMetrics() []provider.CustomMetricInfo {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.customMetrics
}

func (l *WavefrontMetricsLister) ListExternalMetrics() []provider.ExternalMetricInfo {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.externalMetrics
}

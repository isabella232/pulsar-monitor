package cfg

import (
	"fmt"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/kafkaesque-io/pulsar-monitor/src/k8s"
)

const clusterMonInterval = 10 * time.Second

// ClusterHealth a cluster health struct
type ClusterHealth struct {
	sync.RWMutex
	Status         k8s.ClusterStatusCode
	MissingBrokers int
}

var clusterHealth = ClusterHealth{}

// Get gets the cluster health status
func (h *ClusterHealth) Get() (k8s.ClusterStatusCode, int) {
	h.RLock()
	defer h.RUnlock()
	return h.Status, h.MissingBrokers
}

// Set sets the cluster health status
func (h *ClusterHealth) Set(status k8s.ClusterStatusCode, offlineBrokers int) {
	h.Lock()
	h.Status = status
	h.MissingBrokers = offlineBrokers
	h.Unlock()
}

// EvaluateClusterHealth evaluates and reports the k8s cluster health
func EvaluateClusterHealth(client *k8s.Client) error {
	k8sCfg := GetConfig().K8sConfig
	cluster := GetConfig().Name + "-in-cluster"
	// again this is for in-cluster monitoring only

	if err := client.UpdateReplicas(); err != nil {
		return err
	}
	if err := client.WatchPods(k8s.DefaultPulsarNamespace); err != nil {
		return err
	}
	desc, status := client.EvalHealth()
	clusterHealth.Set(status.Status, status.BrokerOfflineInstances)

	PromGaugeInt(GetOfflinePodsCounter(k8sZookeeperSubsystem), cluster, status.ZookeeperOfflineInstances)
	PromGaugeInt(GetOfflinePodsCounter(k8sBookkeeperSubsystem), cluster, status.BookkeeperOfflineInstances)
	PromGaugeInt(GetOfflinePodsCounter(k8sBrokerSubsystem), cluster, status.BrokerOfflineInstances)
	PromGaugeInt(GetOfflinePodsCounter(k8sProxySubsystem), cluster, status.ProxyOfflineInstances)

	if status.Status != k8s.OK {
		errMsg := fmt.Sprintf("cluster %s, k8s pulsar cluster status is unhealthy, error message %s", cluster, desc)
		if status.Status == k8s.TotalDown {
			VerboseAlert(cluster, errMsg, 3*time.Minute)
			ReportIncident(cluster, cluster, "kubernete cluster is down, reported by pulsar-monitor", errMsg, &k8sCfg.AlertPolicy)
		}
	} else {
		ClearIncident(cluster)
	}
	log.Infof("k8s cluster status %v", status)
	return nil
}

// MonitorK8sPulsarCluster start K8sPulsarClusterMonitor thread
func MonitorK8sPulsarCluster() error {
	k8sCfg := GetConfig().K8sConfig
	if !k8sCfg.Enabled {
		return nil
	}

	clientset, err := k8s.GetK8sClient()
	if err != nil {
		log.Errorf("failed to get k8s clientset %v or get pods under pulsar namespace", err)
		return err
	}

	go func(client *k8s.Client) {
		log.Infof("start k8s cluster monitoring ...")
		ticker := time.NewTicker(clusterMonInterval)
		for {
			select {
			case <-ticker.C:
				if err := EvaluateClusterHealth(clientset); err != nil {
					log.Errorf("k8s monitoring failed to watchpods error: %v", err)
				}
			}
		}

	}(clientset)
	return nil
}

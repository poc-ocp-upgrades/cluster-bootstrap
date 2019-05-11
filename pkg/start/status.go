package start

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func waitUntilPodsRunning(ctx context.Context, c kubernetes.Interface, pods map[string][]string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	sc, err := newStatusController(c, pods)
	if err != nil {
		return err
	}
	sc.Run()
	if err := wait.PollImmediateUntil(5*time.Second, sc.AllRunningAndReady, ctx.Done()); err != nil {
		return fmt.Errorf("error while checking pod status: %v", err)
	}
	UserOutput("All self-hosted control plane components successfully started\n")
	return nil
}

type statusController struct {
	client				kubernetes.Interface
	podStore			cache.Store
	watchPodPrefixes	map[string][]string
	lastPodPhases		map[string]*podStatus
}

func newStatusController(client kubernetes.Interface, pods map[string][]string) (*statusController, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return &statusController{client: client, watchPodPrefixes: pods}, nil
}
func (s *statusController) Run() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	options := metav1.ListOptions{}
	podStore, podController := cache.NewInformer(&cache.ListWatch{ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
		return s.client.Core().Pods("").List(options)
	}, WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
		return s.client.Core().Pods("").Watch(options)
	}}, &v1.Pod{}, 30*time.Minute, cache.ResourceEventHandlerFuncs{})
	s.podStore = podStore
	go podController.Run(wait.NeverStop)
}
func (s *statusController) AllRunningAndReady() (bool, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	ps, err := s.podStatus()
	if err != nil {
		glog.Infof("Error retriving pod statuses: %v", err)
		return false, nil
	}
	if s.lastPodPhases == nil {
		s.lastPodPhases = ps
	}
	changed := !reflect.DeepEqual(ps, s.lastPodPhases)
	s.lastPodPhases = ps
	runningAndReady := true
	for p, s := range ps {
		if changed {
			var status string
			switch {
			case s == nil:
				status = "DoesNotExist"
			case s.Phase == v1.PodRunning && s.IsReady:
				status = "Ready"
			case s.Phase == v1.PodRunning && !s.IsReady:
				status = "RunningNotReady"
			default:
				status = string(s.Phase)
			}
			UserOutput("\tPod Status:%24s\t%s\n", p, status)
		}
		if s == nil || s.Phase != v1.PodRunning || !s.IsReady {
			runningAndReady = false
		}
	}
	return runningAndReady, nil
}

type podStatus struct {
	Phase	v1.PodPhase
	IsReady	bool
}

func (s *statusController) podStatus() (map[string]*podStatus, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	status := make(map[string]*podStatus)
	podNames := s.podStore.ListKeys()
	for desc, prefixes := range s.watchPodPrefixes {
		var podName string
	found:
		for _, pn := range podNames {
			for _, prefix := range prefixes {
				if strings.HasPrefix(pn, prefix) {
					podName = pn
					break found
				}
			}
		}
		exists := false
		var p interface{}
		if len(podName) > 0 {
			var err error
			if p, exists, err = s.podStore.GetByKey(podName); err != nil {
				return nil, err
			}
		}
		if !exists {
			status[desc] = nil
			continue
		}
		if p, ok := p.(*v1.Pod); ok {
			status[desc] = &podStatus{Phase: p.Status.Phase}
			for _, c := range p.Status.Conditions {
				if c.Type == v1.PodReady {
					status[desc].IsReady = c.Status == v1.ConditionTrue
				}
			}
		}
	}
	return status, nil
}

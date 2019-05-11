package start

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"github.com/openshift/library-go/pkg/assets/create"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	bootstrapPodsRunningTimeout	= 20 * time.Minute
	assetsCreatedTimeout		= 60 * time.Minute
)

type Config struct {
	AssetDir				string
	PodManifestPath			string
	Strict					bool
	RequiredPodPrefixes		map[string][]string
	WaitForTearDownEvent	string
	EarlyTearDown			bool
}
type startCommand struct {
	podManifestPath			string
	assetDir				string
	strict					bool
	requiredPodPrefixes		map[string][]string
	waitForTearDownEvent	string
	earlyTearDown			bool
}

func NewStartCommand(config Config) (*startCommand, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return &startCommand{assetDir: config.AssetDir, podManifestPath: config.PodManifestPath, strict: config.Strict, requiredPodPrefixes: config.RequiredPodPrefixes, waitForTearDownEvent: config.WaitForTearDownEvent, earlyTearDown: config.EarlyTearDown}, nil
}
func (b *startCommand) Run() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	restConfig, err := clientcmd.BuildConfigFromFlags("", filepath.Join(b.assetDir, assetPathAdminKubeConfig))
	if err != nil {
		return err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	bcp := newBootstrapControlPlane(b.assetDir, b.podManifestPath)
	defer func() {
		if err := bcp.Teardown(); err != nil {
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}()
	defer func() {
		if err != nil {
			UserOutput("Error: %v\n", err)
		}
	}()
	if err = bcp.Start(); err != nil {
		return err
	}
	localClientConfig := rest.CopyConfig(restConfig)
	localClientConfig.Host = "localhost:6443"
	hostURL, err := url.Parse(restConfig.Host)
	if err != nil {
		return err
	}
	localClientConfig.ServerName, _, err = net.SplitHostPort(hostURL.Host)
	if err != nil {
		return err
	}
	createAssetsInBackground := func(ctx context.Context, cancel func(), client *rest.Config) *sync.WaitGroup {
		done := sync.WaitGroup{}
		done.Add(1)
		go func() {
			defer done.Done()
			if err := create.EnsureManifestsCreated(ctx, filepath.Join(b.assetDir, assetPathManifests), client, create.CreateOptions{Verbose: true, StdErr: os.Stderr}); err != nil {
				select {
				case <-ctx.Done():
				default:
					UserOutput("Assert creation failed: %v\n", err)
					cancel()
				}
			}
		}()
		return &done
	}
	ctx, cancel := context.WithTimeout(context.TODO(), bootstrapPodsRunningTimeout)
	defer cancel()
	assetsDone := createAssetsInBackground(ctx, cancel, localClientConfig)
	if err = waitUntilPodsRunning(ctx, client, b.requiredPodPrefixes); err != nil {
		return err
	}
	cancel()
	assetsDone.Wait()
	UserOutput("Sending bootstrap-success event.")
	if _, err := client.CoreV1().Events("kube-system").Create(makeBootstrapSuccessEvent("kube-system", "bootstrap-success")); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	ctx, cancel = context.WithTimeout(context.Background(), assetsCreatedTimeout)
	defer cancel()
	if b.earlyTearDown {
		assetsDone = createAssetsInBackground(ctx, cancel, restConfig)
	} else {
		assetsDone = createAssetsInBackground(ctx, cancel, localClientConfig)
	}
	if len(b.waitForTearDownEvent) != 0 {
		ss := strings.Split(b.waitForTearDownEvent, "/")
		if len(ss) != 2 {
			return fmt.Errorf("tear down event name of format <namespace>/<event-name> expected, got: %q", b.waitForTearDownEvent)
		}
		ns, name := ss[0], ss[1]
		if err := waitForEvent(context.TODO(), client, ns, name); err != nil {
			return err
		}
		UserOutput("Got %s event.", b.waitForTearDownEvent)
	}
	if b.earlyTearDown {
		err = bcp.Teardown()
		bcp = nil
		if err != nil {
			UserOutput("Error tearing down temporary bootstrap control plane: %v\n", err)
		}
	}
	UserOutput("Waiting for remaining assets to be created.\n")
	assetsDone.Wait()
	return nil
}
func UserOutput(format string, a ...interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	fmt.Printf(format, a...)
}
func waitForEvent(ctx context.Context, client kubernetes.Interface, ns, name string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wait.PollImmediateUntil(time.Second, func() (done bool, err error) {
		if _, err := client.CoreV1().Events(ns).Get(name, metav1.GetOptions{}); err != nil && apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			UserOutput("Error waiting for %s/%s event: %v", ns, name, err)
			return false, nil
		}
		return true, nil
	}, ctx.Done())
}
func makeBootstrapSuccessEvent(ns, name string) *corev1.Event {
	_logClusterCodePath()
	defer _logClusterCodePath()
	currentTime := metav1.Time{Time: time.Now()}
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}, InvolvedObject: corev1.ObjectReference{Namespace: ns}, Message: "Required control plane pods have been created", Count: 1, FirstTimestamp: currentTime, LastTimestamp: currentTime}
	return event
}

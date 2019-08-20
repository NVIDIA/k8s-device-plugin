package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"k8s.io/kubernetes/pkg/kubelet/cm/devicemanager/checkpoint"
)

func kubeInit() *kubernetes.Clientset {
	var err error
	var config *rest.Config
	var kubeconfigFile string = os.Getenv("KUBECONFIG")

	if _, err = os.Stat(kubeconfigFile); err != nil {
		klog.V(3).Infof("kubeconfig %s failed to find due to %v", kubeconfigFile, err)
		config, err = rest.InClusterConfig()
		if err != nil {
			klog.Fatalf("Failed due to %v", err)
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigFile)
		if err != nil {
			klog.Fatalf("Failed due to %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed due to %v", err)
	}
	return clientset
}

type controller struct {
	devicePlugin *NvidiaDevicePlugin

	clientset *kubernetes.Clientset
	// podLister can list/get pods from the shared informer's store.
	podLister corelisters.PodLister
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// podInformerSynced returns true if the pod store has been synced at least once.
	podInformerSynced clientgocache.InformerSynced

	// podQueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	//podQueue workqueue.RateLimitingInterface
}

func newController(dp *NvidiaDevicePlugin, kubeClient *kubernetes.Clientset, kubeInformerFactory kubeinformers.SharedInformerFactory, stopCh <-chan struct{}) (*controller, error) {
	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "gpu-topo-device-plugin"})

	c := &controller{
		devicePlugin: dp,
		clientset:    kubeClient,
		//podQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podQueue"),
		recorder: recorder,
	}
	// Create pod informer.
	podInformer := kubeInformerFactory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(clientgocache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *v1.Pod:
				if t.Spec.NodeName != dp.nodeName {
					return false
				}
				return IsGPUTopoPod(t)
			case clientgocache.DeletedFinalStateUnknown:
				if pod, ok := t.Obj.(*v1.Pod); ok {
					if pod.Spec.NodeName != dp.nodeName {
						return false
					}
					return IsGPUTopoPod(pod)
				}
				runtime.HandleError(fmt.Errorf("unable to convert object %T to *v1.Pod in %T", obj, c))
				return false
			default:
				runtime.HandleError(fmt.Errorf("unable to handle object in %T: %T", c, obj))
				return false
			}
		},
		Handler: clientgocache.ResourceEventHandlerFuncs{
			DeleteFunc: c.deletePodFunc,
			UpdateFunc: c.updatePodFunc,
		},
	})

	c.podLister = podInformer.Lister()
	c.podInformerSynced = podInformer.Informer().HasSynced

	// Start informer goroutines.
	go kubeInformerFactory.Start(stopCh)

	if ok := clientgocache.WaitForCacheSync(stopCh, c.podInformerSynced); !ok {
		return nil, fmt.Errorf("failed to wait for pod caches to sync")
	}
	klog.Infoln("init the pod cache successfully")

	return c, nil
}

// Run will set up the event handlers
func (c *controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	klog.Infoln("Starting Topology Controller.")
	klog.Infoln("Waiting for informer caches to sync")

	klog.Infof("Starting %v workers.", threadiness)

	klog.Infoln("Started workers")
	<-stopCh
	klog.Infoln("Shutting down workers")

	return nil
}

func (c *controller) deletePodFunc(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case clientgocache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			klog.Warningf("cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		klog.Warningf("cannot convert to *v1.Pod: %v", t)
		return
	}

	delDevs := strings.Split(pod.Annotations[resourceName], ",")
	klog.V(2).Infof("delete pod %s in ns %s, deleted devs: %v", pod.Name, pod.Namespace, delDevs)
	if err := c.devicePlugin.UpdatePodDevice(nil, delDevs); err != nil {
		klog.Errorf("Failed to update PCI device: %v", err)
	}
	return
}

func (c *controller) updatePodFunc(o, obj interface{}) {

	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	default:
		klog.Warningf("cannot convert to *v1.Pod: %v", t)
		return
	}
	klog.V(2).Infof("add pod[%v]", pod.UID)
	var kubeletDeviceManagerCheckpoint = filepath.Join(pluginapi.DevicePluginPath, "kubelet_internal_checkpoint")
	registeredDevs := make(map[string][]string)
	devEntries := make([]checkpoint.PodDevicesEntry, 0)
	cp := checkpoint.New(devEntries, registeredDevs)
	blob, err := ioutil.ReadFile(kubeletDeviceManagerCheckpoint)
	if err != nil {
		klog.Errorf("Failed to read content from %s: %v", kubeletDeviceManagerCheckpoint, err)
		return
	}
	err = cp.UnmarshalCheckpoint(blob)
	if err != nil {
		klog.Errorf("Failed to unmarshal content: %v", err)
		return
	}
	var env = []string{}
	data, _ := cp.GetData()
	for _, pde := range data {
		if pde.PodUID != string(pod.UID) {
			continue
		}
		for _, devID := range pde.DeviceIDs {
			if val, ok := c.devicePlugin.shadowMap[devID]; ok && val != "" {
				env = append(env, val)
				delete(c.devicePlugin.shadowMap, devID)
			}
		}
	}
	klog.V(2).Infof("Pod[%v] want to be updated: %v", pod.UID, env)
	if len(env) == 0 {
		return
	}
	old := pod.DeepCopy()
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string, 0)
	}
	pod.Annotations[resourceName] = strings.Join(env, ",")
	// update pod annotation
	err = patchPodObject(c.clientset, old, pod)
	if err != nil {
		klog.Error(err)
	}
}

func patchPodObject(c kubernetes.Interface, cur, mod *v1.Pod) error {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, v1.Pod{})
	if err != nil {
		return err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return nil
	}
	klog.V(3).Infof("Patching Pod %s/%s with %s", cur.Namespace, cur.Name, string(patch))
	_, err = c.CoreV1().Pods(cur.Namespace).Patch(cur.Name, types.StrategicMergePatchType, patch)

	return err
}

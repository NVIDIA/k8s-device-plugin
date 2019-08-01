package main

import (
	"fmt"
	"os"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
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
	podQueue workqueue.RateLimitingInterface
}

func newController(kubeClient *kubernetes.Clientset, kubeInformerFactory kubeinformers.SharedInformerFactory, stopCh <-chan struct{}) (*controller, error) {
	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	// eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "gpushare-schd-extender"})

	c := &controller{
		clientset:      kubeClient,
		podQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "podQueue"),
		recorder:       recorder,
		//removePodCache: map[string]*v1.Pod{},
	}
	// Create pod informer.
	podInformer := kubeInformerFactory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(clientgocache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *v1.Pod:
				klog.V(3).Infof("added pod %s in ns %s", t.Name, t.Namespace)
				//return utils.IsGPUsharingPod(t)
				return true
			case clientgocache.DeletedFinalStateUnknown:
				if pod, ok := t.Obj.(*v1.Pod); ok {
					klog.V(3).Infof("delete pod %s in ns %s", pod.Name, pod.Namespace)
					//return utils.IsGPUsharingPod(pod)
				}
				runtime.HandleError(fmt.Errorf("unable to convert object %T to *v1.Pod in %T", obj, c))
				return false
			default:
				runtime.HandleError(fmt.Errorf("unable to handle object in %T: %T", c, obj))
				return false
			}
		},
		Handler: clientgocache.ResourceEventHandlerFuncs{
			//DeleteFunc: c.deletePodFunc,
		},
	})

	c.podLister = podInformer.Lister()
	c.podInformerSynced = podInformer.Informer().HasSynced

	if ok := clientgocache.WaitForCacheSync(stopCh, c.podInformerSynced); !ok {
		return nil, fmt.Errorf("failed to wait for pod caches to sync")
	} else {
		klog.Infoln("info: init the pod cache successfully")
	}

	return c, nil
}

func (c *controller) Start(kubeInformerFactory kubeinformers.SharedInformerFactory, stopCh chan struct{}) {
	// Start informer goroutines.
	go kubeInformerFactory.Start(stopCh)
}

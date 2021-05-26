package main

import (
	"k8s.io/client-go/tools/cache"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	annRequest = "4paradigm.com/vgpu-request"
	annUsing   = "4paradigm.com/vgpu-using"
	annSep     = ","
)

// VDeviceController vdevice id manager
type VDeviceController struct {
	nodeName string
	mux      sync.Mutex
	stopCh   chan struct{}
	idMap    map[string]string
}

// newVDeviceController new VDeviceController
func newVDeviceController(deviceIDs []string) *VDeviceController {
	m := &VDeviceController{
		nodeName: "",
		stopCh:   make(chan struct{}),
		idMap:    make(map[string]string),
	}
	for _, v := range deviceIDs {
		m.idMap[v] = ""
	}
	return m
}

// onAddPod add pod callback func
func (m *VDeviceController) onAddPod(pod *v1.Pod) {
	requestStr := pod.Annotations[annRequest]
	usingStr := pod.Annotations[annUsing]
	if requestStr == "" && usingStr == "" {
		return
	}
	request := strings.Split(requestStr, annSep)
	using := strings.Split(usingStr, annSep)
	if verboseFlag > 5 {
		log.Printf("Debug: using devices %s\n", usingStr)
	}
	m.acquire(request, using)
}

// onUpdatePod update pod callback func
func (m *VDeviceController) onUpdatePod(oldPod, newPod *v1.Pod) {
	oldRequestStr := oldPod.Annotations[annRequest]
	oldUsingStr := oldPod.Annotations[annUsing]
	newRequestStr := newPod.Annotations[annRequest]
	newUsingStr := newPod.Annotations[annUsing]
	if oldRequestStr == newRequestStr && oldUsingStr == newUsingStr {
		return
	}
	if oldRequestStr != "" || oldUsingStr != "" {
		log.Printf("Error: vgpu changed, %s->%s, %s->%s\n", oldRequestStr, newRequestStr, oldUsingStr, newUsingStr)
	}
	newRequest := strings.Split(newRequestStr, annSep)
	newUsing := strings.Split(newUsingStr, annSep)
	if verboseFlag > 5 {
		log.Printf("Debug: using devices %s -> %s\n", oldUsingStr, newUsingStr)
	}
	m.acquire(newRequest, newUsing)
}

// onDeletePod delete pod callback func
func (m *VDeviceController) onDeletePod(pod *v1.Pod) {
	usingStr := pod.Annotations[annUsing]
	if usingStr == "" {
		return
	}
	usingIDs := strings.Split(usingStr, annSep)
	if verboseFlag > 5 {
		log.Printf("Debug: release devices %s\n", usingStr)
	}
	m.release(usingIDs)
}

// initialize initialize vdevice manager
func (m *VDeviceController) initialize() {
	m.nodeName = os.Getenv("NODE_NAME")
	if m.nodeName == "" {
		log.Panicln("Fatal: must set NODE_NAME")
	}
	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		kubeConfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		check(err)
	}
	client, err := kubernetes.NewForConfig(config)
	check(err)
	selector := fields.SelectorFromSet(fields.Set{"spec.nodeName": m.nodeName})
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		client,
		time.Hour*1,
		informers.WithTweakListOptions(
			func(options *metav1.ListOptions) {
				options.FieldSelector = selector.String()
			},
		),
	)

	podInformer := informerFactory.Core().V1().Pods()
	informer := podInformer.Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pod, ok := obj.(*v1.Pod); ok {
				m.onAddPod(pod)
			} else {
				log.Println("Unknown add pod")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod, ok := oldObj.(*v1.Pod)
			if !ok {
				log.Println("Unknown update old pod")
			}
			newPod, ok := newObj.(*v1.Pod)
			if !ok {
				log.Println("Unknown update new pod")
			}
			m.onUpdatePod(oldPod, newPod)
		},
		DeleteFunc: func(obj interface{}) {
			if pod, ok := obj.(*v1.Pod); ok {
				m.onDeletePod(pod)
			} else {
				log.Println("Unknown delete pod")
			}
		},
	},
	)
	m.stopCh = make(chan struct{})
	informerFactory.Start(m.stopCh)
	informerFactory.WaitForCacheSync(m.stopCh)
}

// cleanup finalize vdevice manager
func (m *VDeviceController) cleanup() {
	close(m.stopCh)
}

// available get available device ids
func (m *VDeviceController) available() []string {
	m.mux.Lock()
	defer m.mux.Unlock()
	ids := make([]string, 0, len(m.idMap))
	for k, v := range m.idMap {
		if v == "" {
			ids = append(ids, k)
		}
	}
	return ids
}

// acquire acquire device ids
func (m *VDeviceController) acquire(request, using []string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	for i, v := range using {
		if _, ok := m.idMap[v]; !ok {
			log.Printf("Error: device %s unknown\n", v)
			continue
		}
		if i < len(request) {
			m.idMap[v] = request[i]
		} else {
			log.Printf("Error: %s mismatched\n", v)
			m.idMap[v] = "mismatched"
		}
	}
}

// release release device  ids
func (m *VDeviceController) release(using []string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	for _, v := range using {
		if _, ok := m.idMap[v]; ok {
			m.idMap[v] = ""
		} else {
			log.Printf("Error: device %s unknown\n", v)
		}
	}
}

// releaseByRequest release device ids by request ids
func (m *VDeviceController) releaseByRequest(request []string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	for k, v := range m.idMap {
		for _, r := range request {
			if v == r {
				log.Printf("Error: device %s[%s] loss.\n", k, v)
				m.idMap[k] = ""
			}
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/urfave/cli/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	NvidiaResourceName      = "nvidia.com/gpu"
	NvidiaMigResourcePrefix = "nvidia.com/mig-"
)

type webhook struct {
	decoder admission.Decoder
}

func newWebHook() (*admission.Webhook, error) {
	logf.SetLogger(klog.NewKlogr())
	schema := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(schema); err != nil {
		return nil, err
	}
	decoder := admission.NewDecoder(schema)
	wh := &admission.Webhook{Handler: &webhook{decoder: decoder}}
	return wh, nil
}

func requiresGPU(pod *corev1.Pod) bool {
	for _, c := range pod.Spec.Containers {
		for key := range c.Resources.Limits {
			if string(key) == NvidiaResourceName || strings.HasPrefix(string(key), NvidiaMigResourcePrefix) {
				return true
			}
		}
	}
	return false
}

func (h *webhook) Handle(_ context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := h.decoder.Decode(req, pod)
	if err != nil {
		klog.Errorf("Failed to decode request: %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if len(pod.Spec.Containers) == 0 {
		klog.Warningf("Denying admission as pod has no containers, pod namespace: %s, pod name: %s", pod.Namespace, pod.Name)
		return admission.Denied("pod has no containers")
	}
	name := pod.Name
	if name == "" {
		name = pod.GenerateName
	}

	klog.Infof("pod namespace: %s, pod name: %s", pod.Namespace, name)

	if !requiresGPU(pod) {
		return admission.Allowed("")
	}

	if pod.Spec.RuntimeClassName == nil {
		runtimeClass := "nvidia"
		pod.Spec.RuntimeClassName = &runtimeClass
		marshaledPod, err := json.Marshal(pod)
		if err != nil {
			klog.Errorf("Failed to marshal pod, pod namespace: %s, pod name: %s, error: %v", pod.Namespace, pod.Name, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
	}
	return admission.Allowed("")
}

func main() {
	app := &cli.App{
		Name:  "gpu-runtime-webhook",
		Usage: "Mutating admission webhook to inject nvidia RuntimeClass",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "cert",
				Usage: "path to TLS cert file",
				Value: "/tls/tls.crt",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "path to TLS key file",
				Value: "/tls/tls.key",
			},
			&cli.StringFlag{
				Name:  "addr",
				Usage: "webhook listen address",
				Value: ":8443",
			},
		},
		Action: func(c *cli.Context) error {
			tlsCertFile := c.String("cert")
			tlsKeyFile := c.String("key")
			addr := c.String("addr")

			wh, err := newWebHook()
			if err != nil {
				klog.ErrorS(err, "Failed to create webhook")
				return err
			}

			router := httprouter.New()
			router.Handler("POST", "/webhook", wh)
			router.HandlerFunc("GET", "/healthz", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			if tlsCertFile == "" || tlsKeyFile == "" {
				return fmt.Errorf("both cert and key must be provided for TLS")
			}
			return http.ListenAndServeTLS(addr, tlsCertFile, tlsKeyFile, router)
		},
	}

	klog.Infof("Nvidia pod webhook server starting...")
	if err := app.Run(os.Args); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}

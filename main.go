package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/StatCan/daaas-aaw-toleration-injector/pkg/signals"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	masterURL  string
	kubeconfig string
)

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, world!")
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok")
}

func handleMutate(namespacesLister corev1listers.NamespaceLister) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Decode the request
		body, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%s", err)
			return
		}

		admissionReview := v1beta1.AdmissionReview{}
		if err := json.Unmarshal(body, &admissionReview); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%s", err)
			return
		}

		response, err := mutate(namespacesLister, *admissionReview.Request)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%s", err)
			return
		}

		reviewResponse := v1beta1.AdmissionReview{
			Response: &response,
		}

		if body, err = json.Marshal(reviewResponse); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%s", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	stopCh := signals.SetupSignalHandler()

	// We need to setup a Kubernetes client in order to maintain a list of namespaces.
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("error building kubeconfig: %v", err)
	}

	kubeclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("error building kubernetes client: %v", err)
	}

	informerFactory := informers.NewSharedInformerFactory(kubeclient, time.Second*30)
	namespacesInformer := informerFactory.Core().V1().Namespaces()
	namespacesLister := namespacesInformer.Lister()

	informerFactory.Start(stopCh)

	// Wait for caches to sync
	klog.Infof("waiting for synchronization of informer caches")
	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(time.Second*30))
	if ok := cache.WaitForCacheSync(ctx.Done(), namespacesInformer.Informer().HasSynced); !ok {
		klog.Fatalf("failed to synchronize informer caches")
	}
	klog.Infof("informer caches have synchronized")

	// Start the webserver
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/_healthz", handleHealthz)
	mux.HandleFunc("/mutate", handleMutate(namespacesLister))

	s := &http.Server{
		Addr:           ":8443",
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println("Listening on :8443")
	log.Fatal(s.ListenAndServeTLS("./certs/tls.crt", "./certs/tls.key"))
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

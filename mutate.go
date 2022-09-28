package main

import (
	"encoding/json"
	"fmt"
	"log"

	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

func mutate(namespacesLister corev1listers.NamespaceLister, request v1beta1.AdmissionRequest, namespaceConfig map[string][]string) (v1beta1.AdmissionResponse, error) {
	response := v1beta1.AdmissionResponse{}

	// Default response
	response.Allowed = true
	response.UID = request.UID

	// Decode the pod object
	pod := v1.Pod{}
	if err := json.Unmarshal(request.Object.Raw, &pod); err != nil {
		return response, fmt.Errorf("unable to decode Pod %w", err)
	}

	log.Printf("Identifying if toleration injection should be applied to request %s/%s", request.Namespace, pod.Name)

	inject := true
	for _, toleration := range pod.Spec.Tolerations {
		if toleration.Key == "CriticalAddonsEarly" || toleration.Key == "node.statcan.gc.ca/purpose" {
			klog.Infof("not injecting pod %s/%s, appropriate tolerations already exist", request.Namespace, pod.Name)
			inject = false
			break
		}
	}

	if inject {
		namespace, err := namespacesLister.Get(request.Namespace)
		if err != nil {
			response.Result = &metav1.Status{
				Status: metav1.StatusFailure,
			}
			return response, fmt.Errorf("unable to find namespace %q", request.Namespace)
		}

		tolerations := []v1.Toleration{}
		purpose := ""
		ok := false
		if purpose, ok = namespace.ObjectMeta.Labels["namespace.statcan.gc.ca/purpose"]; !ok {
			purpose = "user"
		}

		// Schedule `daaas` namespaces on `system` nodes
		if purpose == "daaas" {
			purpose = "system"
		}

		tolerations = append(tolerations, v1.Toleration{
			Key:      "node.statcan.gc.ca/purpose",
			Value:    purpose,
			Operator: v1.TolerationOpEqual,
			Effect:   v1.TaintEffectNoSchedule,
		})

		// Check for a GPU
		numGPU := 0
		for _, container := range pod.Spec.Containers {
			// if container.Resources.Requests.
			if limit, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
				if !limit.IsZero() {
					numGPU = int(limit.Value())
					break
				}
			}
		}

		// Check for CPU request values -> pods can have many containers requesting different CPU resources, need to select max(requests)
		numCPU := 0
		for _, container := range pod.Spec.Containers {
			if req, ok := container.Resources.Requests["cpu"]; ok {
				if numCPU < int(req.Value()) {
					numCPU = int(req.Value())
				}
			}
		}

		log.Printf("Max CPU/GPU requested: %d/%d", numCPU, numGPU)

		// conditional eval
		if numGPU != 0 {
			if numGPU == 1 {
				tolerations = append(tolerations, v1.Toleration{
					Key:      "node.statcan.gc.ca/use",
					Value:    "gpu",
					Operator: v1.TolerationOpEqual,
					Effect:   v1.TaintEffectNoSchedule,
				})
			} else if numGPU == 4 {
				tolerations = append(tolerations, v1.Toleration{
					Key:      "node.statcan.gc.ca/use",
					Value:    "gpu-4",
					Operator: v1.TolerationOpEqual,
					Effect:   v1.TaintEffectNoSchedule,
				})
			}
		} else if numCPU > 14 && numCPU <= 72 {
			bigCpuNamespaces := namespaceConfig["bigCPUns"]
			log.Printf("request.Namespace: %s", request.Namespace)
			for _, ns := range bigCpuNamespaces { // store namespaces in map[ns string] string for indexablility if needed
				log.Printf("ns: %s", ns)
				if request.Namespace == ns {
					tolerations = append(tolerations, v1.Toleration{
						Key:      "node.statcan.gc.ca/use",
						Value:    "cpu-72",
						Operator: v1.TolerationOpEqual,
						Effect:   v1.TaintEffectNoSchedule,
					})
				}
			}
		} else if request.Namespace == "cloud-main-system" {
			/*
				If the pod namespace is "cloud-main-system", then it is an istio egress gateway pod that
				should be scheduled to the `cloud-main-system` node pool. This node pool specifically has
				the taint `node.statcan.gc.ca/use=cloud-main-system`, so this block is adding the corresponding
				toleration.
			*/
			tolerations = append(tolerations, v1.Toleration{
				Key:      "node.statcan.gc.ca/use",
				Value:    "cloud-main-system",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			})
		} else {
			tolerations = append(tolerations, v1.Toleration{
				Key:      "node.statcan.gc.ca/use",
				Value:    "general",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			})
		}

		// System pools are always protected-b
		if purpose == "system" {
			tolerations = append(tolerations, v1.Toleration{
				Key:      "data.statcan.gc.ca/classification",
				Value:    "protected-b",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			})
		} else {
			if classification, ok := pod.ObjectMeta.Labels["data.statcan.gc.ca/classification"]; ok {
				tolerations = append(tolerations, v1.Toleration{
					Key:      "data.statcan.gc.ca/classification",
					Value:    classification,
					Operator: v1.TolerationOpEqual,
					Effect:   v1.TaintEffectNoSchedule,
				})
			} else {
				tolerations = append(tolerations, v1.Toleration{
					Key:      "data.statcan.gc.ca/classification",
					Value:    "unclassified",
					Operator: v1.TolerationOpEqual,
					Effect:   v1.TaintEffectNoSchedule,
				})
			}
		}

		patch := v1beta1.PatchTypeJSONPatch
		response.PatchType = &patch

		patches := []map[string]interface{}{}

		for _, toleration := range tolerations {
			patches = append(patches, map[string]interface{}{
				"op":    "add",
				"path":  "/spec/tolerations/-",
				"value": toleration,
			})
		}

		response.Patch, err = json.Marshal(patches)
		if err != nil {
			return response, err
		}

		response.Result = &metav1.Status{
			Status: metav1.StatusSuccess,
		}
	}

	return response, nil
}

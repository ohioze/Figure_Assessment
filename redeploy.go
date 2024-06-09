package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// Parse flags
	kubeconfig := flag.String("kubeconfig", "", "path to Kubernetes config file")
	namespace := flag.String("namespace", "", "Kubernetes namespace (optional)")
	flag.Parse()

	// Build Kubernetes client config
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// List all pods in the specified namespace (or all namespaces if none is specified)
	listOptions := metav1.ListOptions{}
	pods, err := clientset.CoreV1().Pods(*namespace).List(context.Background(), listOptions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing pods: %v\n", err)
		os.Exit(1)
	}

	// Find and restart deployments or stateful sets with "database" in the pod name
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, "database") {
			if ownerRefs := pod.OwnerReferences; len(ownerRefs) > 0 {
				for _, ownerRef := range ownerRefs {
					switch ownerRef.Kind {
					case "Deployment":
						err = restartDeployment(clientset, pod.Namespace, ownerRef.Name)
					case "StatefulSet":
						err = restartStatefulSet(clientset, pod.Namespace, ownerRef.Name)
					}
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error restarting %s %s: %v\n", ownerRef.Kind, ownerRef.Name, err)
					} else {
						fmt.Printf("Successfully restarted %s %s\n", ownerRef.Kind, ownerRef.Name)
					}
				}
			}
		}
	}
}

func restartDeployment(clientset *kubernetes.Clientset, namespace, name string) error {
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	deployment, err := deploymentsClient.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Update the deployment's annotations to trigger a rollout restart
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().String()

	_, err = deploymentsClient.Update(context.Background(), deployment, metav1.UpdateOptions{})
	return err
}

func restartStatefulSet(clientset *kubernetes.Clientset, namespace, name string) error {
	statefulSetsClient := clientset.AppsV1().StatefulSets(namespace)
	statefulSet, err := statefulSetsClient.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Update the stateful set's annotations to trigger a rollout restart
	if statefulSet.Spec.Template.Annotations == nil {
		statefulSet.Spec.Template.Annotations = make(map[string]string)
	}
	statefulSet.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().String()

	_, err = statefulSetsClient.Update(context.Background(), statefulSet, metav1.UpdateOptions{})
	return err
}

package client

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig string
	Clientset  *kubernetes.Clientset
)

func init() {
	kubeconfig := os.Getenv("KUBECONFIG")

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	Clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

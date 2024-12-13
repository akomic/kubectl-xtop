package client

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	Clientset *kubernetes.Clientset
	Config    *rest.Config
)

func init() {
	// Use the default kubeconfig loading rules
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get a rest.Config from the kubeconfig file
	var err error
	Config, err = kubeConfig.ClientConfig()
	if err != nil {
		panic(err.Error())
	}

	// Create the clientset
	Clientset, err = kubernetes.NewForConfig(Config)
	if err != nil {
		panic(err.Error())
	}
}

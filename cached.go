package kube

import (
	"fmt"
	"log"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type CachedClient struct {
	mu            *sync.Mutex
	clientset     *kubernetes.Clientset
	restConfig    *rest.Config
	stopChan      chan struct{}
	defaultResync time.Duration
	factory       informers.SharedInformerFactory
	podInformer   v1.PodInformer
	pods          map[string]*corev1.Pod
}

func NewCachedFromIncluster() (*CachedClient, error) {
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return &CachedClient{
		clientset:     clientset,
		restConfig:    c,
		stopChan:      make(chan struct{}),
		defaultResync: 0,
		mu:            &sync.Mutex{},
	}, nil
}

func NewCachedFromKubeConfig(kubeConfigPath string) (*CachedClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c := &CachedClient{
		clientset:     clientset,
		restConfig:    config,
		stopChan:      make(chan struct{}),
		defaultResync: 0,
		factory:       informers.NewSharedInformerFactory(clientset, 0),
		pods:          make(map[string]*corev1.Pod),
		mu:            &sync.Mutex{},
	}
	c.startFactory()
	c.initPodsCache()
	return c, nil
}
func (c *CachedClient) startFactory() {
	c.podInformer = c.factory.Core().V1().Pods()
	log.Println("started factory")
	go c.factory.Start(c.stopChan)
}

func (c *CachedClient) initPodsCache() {
	log.Println("Initializing pods cache")
	informer := c.podInformer.Informer()
	if !cache.WaitForCacheSync(c.stopChan, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.podAdd,
		UpdateFunc: c.podUpdate,
		DeleteFunc: c.podDelete,
	})
	log.Println("cache has been initialized")
}

func (c *CachedClient) Wait() {
	<-c.stopChan
}

func (c *CachedClient) podAdd(obj interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pod := obj.(*corev1.Pod)
	log.Printf("Adding new pod %s\n", pod.Name)
	c.pods[pod.Name] = pod
}
func (c *CachedClient) podUpdate(old, updated interface{}) {
	oldPod := old.(*corev1.Pod)
	newPod := updated.(*corev1.Pod)
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Printf("Updating pod: %s %s", oldPod.Name, newPod.Status.Message)
	c.pods[oldPod.Name] = newPod
}
func (c *CachedClient) podDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)
	log.Printf("deleting pod: %s", pod.Name)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pods, pod.Name)
}
func (c *CachedClient) GetPods() ([]*corev1.Pod, error) {
	lister := c.podInformer.Lister().Pods("default")

	pods, err := lister.List(labels.Everything())
	return pods, err

}

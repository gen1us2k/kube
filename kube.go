package kube

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/davecgh/go-spew/spew"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/rest"
)

const (
	defaultApiUriPath  = "/api"
	defaultApisUriPath = "/apis"
)

type Client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

func NewFromIncluster() (*Client, error) {
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: clientset, restConfig: c}, nil
}

func NewFromKubeConfig(kubeConfigPath string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: clientset, restConfig: config}, nil
}
func (c *Client) resourceClient(gv schema.GroupVersion) (rest.Interface, error) {
	cfg := c.restConfig
	cfg.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	cfg.GroupVersion = &gv
	if len(gv.Group) == 0 {
		cfg.APIPath = defaultApiUriPath
	} else {
		cfg.APIPath = defaultApisUriPath
	}
	return rest.RESTClientFor(cfg)
}

func (c *Client) Apply(files []string) error {
	groupResources, err := restmapper.GetAPIGroupResources(c.clientset.Discovery())
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		fBytes, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}
		objs, err := c.getObjects(fBytes)
		if err != nil {
			return err
		}
		spew.Dump(objs)
		for i := range objs {
			// Get some metadata needed to make the REST request.
			gvk := objs[i].GetObjectKind().GroupVersionKind()
			gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
			mapping, err := mapper.RESTMapping(gk, gvk.Version)
			if err != nil {
				return err
			}
			namespace, name, err := retrievesMetaFromObject(objs[i])
			if err != nil {
				return err
			}
			cli, err := c.resourceClient(mapping.GroupVersionKind.GroupVersion())
			if err != nil {
				return err
			}
			helper := resource.NewHelper(cli, mapping)
			err = applyObject(helper, namespace, name, objs[i])
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func retrievesMetaFromObject(obj runtime.Object) (namespace, name string, err error) {
	name, err = meta.NewAccessor().Name(obj)
	if err != nil {
		return
	}
	namespace, err = meta.NewAccessor().Namespace(obj)
	if err != nil {
		return
	}
	return
}

func applyObject(helper *resource.Helper, namespace, name string, obj runtime.Object) error {
	if _, err := helper.Get(namespace, name); err != nil {
		_, err = helper.Create(namespace, false, obj)
		if err != nil {
			return err
		}
	} else {
		_, err = helper.Replace(namespace, name, true, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) getObjects(f []byte) ([]runtime.Object, error) {
	objs := make([]runtime.Object, 0)
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(f), 100)
	var err error
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}

		objs = append(objs, &unstructured.Unstructured{Object: unstructuredMap})
	}

	return objs, nil
}

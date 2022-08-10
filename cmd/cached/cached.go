package main

import (
	"flag"
	"fmt"

	"log"

	"github.com/gen1us2k/kube"
)

func main() {
	var file string
	flag.StringVar(&file, "f", "", "file to apply")
	flag.Parse()
	k, err := kube.NewCachedFromKubeConfig("/Users/gen1us2k/.kube/config")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Getting pods")
	pods, err := k.GetPods()
	if err != nil {
		log.Fatal(err)
	}
	for _, pod := range pods {
		fmt.Println(pod.Name)
	}
	k.Wait()

}

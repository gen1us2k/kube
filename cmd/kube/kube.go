package main

import (
	"flag"

	"log"

	"github.com/gen1us2k/kube"
)

func main() {
	var file string
	flag.StringVar(&file, "f", "", "file to apply")
	flag.Parse()
	k, err := kube.NewFromKubeConfig("/Users/gen1us2k/.kube/config")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(k.Apply([]string{file}))

}

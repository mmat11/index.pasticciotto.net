package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"flag"
	"html/template"
	"log"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

//go:embed static/index.jpg
var picture []byte

//go:embed index.tmpl
var indexT string

type tData struct {
	Picture string
	Data    map[string]map[string][]string
}

func main() {
	var (
		ctx  = context.TODO()
		data = make(map[string]map[string][]string)
	)

	t, err := template.New("webpage").Parse(indexT)
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("create clientset: %v", err)
	}

	nss, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("get namespaces: %v", err)
	}
	for _, ns := range nss.Items {
		data[ns.Name] = make(map[string][]string)

		igs, err := cs.NetworkingV1().Ingresses(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Fatalf("get ingresses for namespace %s: %v", ns.Name, err)
		}

		for _, ig := range igs.Items {
			for _, rule := range ig.Spec.Rules {
				data[ns.Name][ig.Name] = append(data[ns.Name][ig.Name], rule.Host)
			}
		}
	}

	d := tData{
		Picture: base64.StdEncoding.EncodeToString(picture),
		Data:    data,
	}

	http.HandleFunc("/", handler(t, d))
	log.Println("listening on http://0.0.0.0:8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func handler(t *template.Template, d tData) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Execute(w, d)
	}
}

package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"flag"
	"html/template"
	"log"
	"net/http"
	"time"

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
	Updated time.Time
}

func main() {
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

	td := tData{
		Picture: base64.StdEncoding.EncodeToString(picture),
		Data:    make(map[string]map[string][]string),
	}
	update(cs, &td)

	ticker := time.NewTicker(30 * time.Second)
	quit := make(chan struct{}, 1)
	go func(cs *kubernetes.Clientset, td *tData) {
		for {
			select {
			case <-ticker.C:
				log.Println("updating...")
				update(cs, td)
			case <-quit:
				return
			}
		}
	}(cs, &td)

	http.HandleFunc("/", handler(t, &td))
	log.Println("listening on http://0.0.0.0:8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		quit <- struct{}{}
		log.Fatal(err)
	}
}

func handler(t *template.Template, td *tData) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Execute(w, td)
	}
}

func update(cs *kubernetes.Clientset, td *tData) {
	ctx := context.Background()

	nss, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("get namespaces: %v\n", err)
		return
	}
	for _, ns := range nss.Items {
		td.Data[ns.Name] = make(map[string][]string)

		igs, err := cs.NetworkingV1().Ingresses(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("get ingresses for namespace %s: %v\n", ns.Name, err)
			return
		}

		for _, ig := range igs.Items {
			for _, rule := range ig.Spec.Rules {
				td.Data[ns.Name][ig.Name] = append(td.Data[ns.Name][ig.Name], rule.Host)
			}
		}
	}
	td.Updated = time.Now().Local()
}

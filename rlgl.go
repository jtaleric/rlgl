package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	t := flag.Int("t", 10, "Time in Minutes to look back for events.")
	flag.Parse()

	kconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{})
	rconfig, err := kconfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	coreclient, err := corev1.NewForConfig(rconfig)
	if err != nil {
		panic(err)
	}
	if CheckEvents(coreclient) && CheckNodeEvents(coreclient, t) {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}

func CheckNodeEvents(c *corev1.CoreV1Client, d *int) bool {
	t := metav1.Now().Time.Add((-time.Duration(*d)) * time.Minute)
	fmt.Printf("👀 Looking for Node troublesome events (now()-%dm) in the cluster..\r\n", *d)
	f, err := fields.ParseSelector("type!=Normal,involvedObject.kind=Node")
	if err != nil {
		fmt.Printf("Unable to build fieldSelector : %s \r\n", err)
		return false
	}
	ev, err := c.Events(metav1.NamespaceAll).List(context.TODO(),
		metav1.ListOptions{
			FieldSelector: f.String(),
		})
	if err != nil {
		fmt.Println("Unable to retrieve events.")
		return false
	}
	var tchk v1.EventList
	// Check the lastTimestamp
	for _, e := range ev.Items {
		if e.LastTimestamp.Time.After(t) {
			tchk.Items = append(tchk.Items, e)
		}
	}
	if len(tchk.Items) > 0 {
		fmt.Println("🔥 Detected troublesome Node events :")
		BadEvents(tchk.Items)
		return false
	}
	return true
}

func CheckEvents(c *corev1.CoreV1Client) bool {
	fmt.Println("👀 Looking for general troublesome events in the cluster..")
	f, err := fields.ParseSelector("type!=Normal,type!=Warning")
	if err != nil {
		fmt.Printf("Unable to build fieldSelector : %s\r\n", err)
		return false
	}
	ev, err := c.Events(metav1.NamespaceAll).List(context.TODO(),
		metav1.ListOptions{
			FieldSelector: f.String(),
		})
	if err != nil {
		fmt.Println("Unable to retrieve events.")
		return false
	}
	if len(ev.Items) > 0 {
		fmt.Println("🔥 Detected troublesome events: ")
		BadEvents(ev.Items)
		return false
	}
	return true
}

func BadEvents(i []v1.Event) {
	for _, e := range i {
		fmt.Printf("\t🗒️  %s \t %s \t %s \r\n", e.Type, e.Reason, e.LastTimestamp)
	}
}

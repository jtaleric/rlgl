package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const followHelp string = `Follow: exit until an error is found, keep checking every N seconds.
Use ctrl + c to force exit.`

func main() {
	t := flag.Int("t", 10, "Time in Minutes to look back for events.")
	f := flag.Bool("f", false, followHelp)
	sleep := flag.Int("sleep", 30, "Time in Seconds to sleep before next check.")
	background := flag.Bool("background", false, "Run in the background")
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

	result := CheckEvents(coreclient) && CheckNodeEvents(coreclient, t)
	if result {
		fmt.Println("No troublesome events found.")
	} else {
		os.Exit(1)
	}

	if *background {
		fmt.Println("Program is running in the background...")
		fmt.Println("Use 'kill <pid>' to stop the program.")
		// Run the program in the background
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
	} else {
		for *f {
			time.Sleep(time.Duration(*sleep) * time.Second)
			result = CheckEvents(coreclient) && CheckNodeEvents(coreclient, t)
			if result {
				os.Exit(0)
			}
		}
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
		fmt.Println("🔥 Detected troublesome events:")
		BadEvents(ev.Items)
		return false
	}
	return true
}

func BadEvents(i []v1.Event) {
	for _, e := range i {
		fmt.Printf("🗒️  %s \t %s \t %s \n", e.Type, e.Reason, e.LastTimestamp)
		fmt.Printf("Message: \t %s \n", e.Message)
		fmt.Printf("Namespace: \t %s \n", e.InvolvedObject.Namespace)
		fmt.Printf("Event Time: \t %s \n", e.EventTime)
		fmt.Println("------------------------")
	}
}

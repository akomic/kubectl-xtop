/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	client "github.com/akomic/kubectl-xtop/client"
	v1 "k8s.io/api/core/v1"
	"github.com/spf13/cobra"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	podSortBy string
	namespace string
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "Top pods",
	Run: func(cmd *cobra.Command, args []string) {
		runPodsCommand()
	},
}

type podColumn struct {
	header string
	getter func(podInfo) string
}

type podInfo struct {
	name       string
	namespace  string
	nodeName   string
	resources  map[string]*resource.Quantity
	phase      string
}

func toPodColumnName(key string) string {
	for i := 1; i < len(key); i++ {
		if key[i] >= 'A' && key[i] <= 'Z' {
			return strings.ToUpper(key[:i]) + " " + strings.ToUpper(key[i:])
		}
	}
	return strings.ToUpper(key)
}

var podColumns []podColumn

type podInfoList []podInfo

func (p podInfoList) Len() int      { return len(p) }
func (p podInfoList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p podInfoList) Less(i, j int) bool {
	sortMap := map[string]string{
		"cpu-req":   "cpuReq",
		"cpu-limit": "cpuLimit",
		"mem-req":   "memReq",
		"mem-limit": "memLimit",
	}
	
	if resourceKey, ok := sortMap[podSortBy]; ok {
		return p[i].resources[resourceKey].Cmp(*p[j].resources[resourceKey]) < 0
	}
	return p[i].name < p[j].name
}

func runPodsCommand() {
	// Get pods
	var pods *v1.PodList
	var err error
	
	if namespace != "" {
		pods, err = client.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	} else {
		pods, err = client.Clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	}
	
	if err != nil {
		panic(err.Error())
	}

	// Convert to podInfo list
	podsList := make(podInfoList, 0, len(pods.Items))
	
	for _, pod := range pods.Items {
		resources := map[string]*resource.Quantity{
			"cpuReq":   resource.NewQuantity(0, resource.DecimalSI),
			"cpuLimit": resource.NewQuantity(0, resource.DecimalSI),
			"memReq":   resource.NewQuantity(0, resource.BinarySI),
			"memLimit": resource.NewQuantity(0, resource.BinarySI),
		}

		// Sum up container resources
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if val, ok := container.Resources.Requests[v1.ResourceCPU]; ok {
					resources["cpuReq"].Add(val)
				}
				if val, ok := container.Resources.Requests[v1.ResourceMemory]; ok {
					resources["memReq"].Add(val)
				}
			}

			if container.Resources.Limits != nil {
				if val, ok := container.Resources.Limits[v1.ResourceCPU]; ok {
					resources["cpuLimit"].Add(val)
				}
				if val, ok := container.Resources.Limits[v1.ResourceMemory]; ok {
					resources["memLimit"].Add(val)
				}
			}
		}

		podsList = append(podsList, podInfo{
			name:      pod.Name,
			namespace: pod.Namespace,
			nodeName:  pod.Spec.NodeName,
			resources: resources,
			phase:     string(pod.Status.Phase),
		})
	}

	// Sort the slice
	sort.Sort(podsList)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
	printPodTable(w, podsList)
}

func printPodTable(w *tabwriter.Writer, podsList podInfoList) {
	// Print headers
	fmt.Fprintln(w, strings.Join(getPodRowValues(podInfo{}, true), "\t"))

	// Print rows
	for _, pod := range podsList {
		fmt.Fprintln(w, strings.Join(getPodRowValues(pod, false), "\t"))
	}
	w.Flush()
}

func getPodRowValues(pod podInfo, isHeader bool) []string {
	values := make([]string, len(podColumns))
	for i, col := range podColumns {
		if isHeader {
			values[i] = col.header
		} else {
			values[i] = col.getter(pod)
		}
	}
	return values
}

func init() {
	// Initialize base columns
	podColumns = []podColumn{
		{
			header: "NAMESPACE",
			getter: func(pod podInfo) string {
				return pod.namespace
			},
		},
		{
			header: "NAME",
			getter: func(pod podInfo) string {
				return pod.name
			},
		},
		{
			header: "NODE",
			getter: func(pod podInfo) string {
				return pod.nodeName
			},
		},
		{
			header: "STATUS",
			getter: func(pod podInfo) string {
				return pod.phase
			},
		},
	}

	// Add resource columns dynamically
	resourceKeys := []string{"cpuReq", "cpuLimit", "memReq", "memLimit"}
	for _, key := range resourceKeys {
		col := podColumn{
			header: toPodColumnName(key),
			getter: func(key string) func(pod podInfo) string {
				return func(pod podInfo) string {
					val, suffix := pod.resources[key].CanonicalizeBytes(make([]byte, 0, 100))
					return string(val) + string(suffix)
				}
			}(key),
		}
		podColumns = append(podColumns, col)
	}

	rootCmd.AddCommand(podsCmd)
	podsCmd.Flags().StringVar(&podSortBy, "sort-by", "name", "Sort pods by: name, cpu-req, cpu-limit, mem-req, mem-limit")
	podsCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "If present, show pods in the specified namespace only")
}

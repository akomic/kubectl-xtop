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
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	podSortBy string
	namespace string
	verbose   bool
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
	cpuUsage   *resource.Quantity
	memUsage   *resource.Quantity
}

func toPodColumnName(key string) string {
	for i := 1; i < len(key); i++ {
		if key[i] >= 'A' && key[i] <= 'Z' {
			return strings.ToUpper(key[:i]) + " " + strings.ToUpper(key[i:])
		}
	}
	return strings.ToUpper(key)
}

var podColumns = []podColumn{
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
		header: "STATUS",
		getter: func(pod podInfo) string {
			return pod.phase
		},
	},
}

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
	// Add NODE column if verbose flag is set
	if verbose {
		nodeColumn := podColumn{
			header: "NODE",
			getter: func(pod podInfo) string {
				return pod.nodeName
			},
		}
		podColumns = append(podColumns, nodeColumn)
	}
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

	// Get metrics client
	metricsClient, err := metrics.NewForConfig(client.Config)
	if err != nil {
		panic(err.Error())
	}

	// Get pod metrics
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Warning: Could not fetch metrics: %v\n", err)
	}

	// Create metrics lookup map
	metricsMap := make(map[string]map[string]*resource.Quantity)
	if podMetrics != nil {
		for _, podMetric := range podMetrics.Items {
			key := fmt.Sprintf("%s/%s", podMetric.Namespace, podMetric.Name)
			metricsMap[key] = map[string]*resource.Quantity{
				"cpu":    resource.NewQuantity(0, resource.DecimalSI),
				"memory": resource.NewQuantity(0, resource.BinarySI),
			}
			
			for _, container := range podMetric.Containers {
				metricsMap[key]["cpu"].Add(container.Usage[v1.ResourceCPU])
				metricsMap[key]["memory"].Add(container.Usage[v1.ResourceMemory])
			}
		}
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

		info := podInfo{
			name:      pod.Name,
			namespace: pod.Namespace,
			nodeName:  pod.Spec.NodeName,
			resources: resources,
			phase:     string(pod.Status.Phase),
			cpuUsage:  resource.NewQuantity(0, resource.DecimalSI),
			memUsage:  resource.NewQuantity(0, resource.BinarySI),
		}

		// Add metrics if available
		key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		if metrics, ok := metricsMap[key]; ok {
			info.cpuUsage = metrics["cpu"]
			info.memUsage = metrics["memory"]
		}

		podsList = append(podsList, info)
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

	// Add resource columns dynamically
	resourceKeys := []string{"cpuReq", "cpuLimit", "cpuUsage (%)", "memReq", "memLimit", "memUsage (%)"}
	for _, key := range resourceKeys {
		col := podColumn{
			header: toPodColumnName(key),
			getter: func(key string) func(pod podInfo) string {
				return func(pod podInfo) string {
					var quantity *resource.Quantity
					switch key {
					case "cpuUsage (%)":
						if pod.cpuUsage == nil {
							return "<none>"
						}
						if pod.resources["cpuReq"] == nil || pod.resources["cpuReq"].IsZero() {
							val, suffix := pod.cpuUsage.CanonicalizeBytes(make([]byte, 0, 100))
							return string(val) + string(suffix)
						}
						percentage := float64(pod.cpuUsage.MilliValue()) / float64(pod.resources["cpuReq"].MilliValue()) * 100
						val, suffix := pod.cpuUsage.CanonicalizeBytes(make([]byte, 0, 100))
						return fmt.Sprintf("%s%s (%.0f%%)", string(val), string(suffix), percentage)
					case "memUsage (%)":
						if pod.memUsage == nil {
							return "<none>"
						}
						if pod.resources["memReq"] == nil || pod.resources["memReq"].IsZero() {
							val, suffix := pod.memUsage.CanonicalizeBytes(make([]byte, 0, 100))
							return string(val) + string(suffix)
						}
						percentage := float64(pod.memUsage.Value()) / float64(pod.resources["memReq"].Value()) * 100
						val, suffix := pod.memUsage.CanonicalizeBytes(make([]byte, 0, 100))
						return fmt.Sprintf("%s%s (%.0f%%)", string(val), string(suffix), percentage)
					default:
						quantity = pod.resources[key]
						if quantity == nil {
							return "<none>"
						}
						val, suffix := quantity.CanonicalizeBytes(make([]byte, 0, 100))
						return string(val) + string(suffix)
					}
				}
			}(key),
		}
		podColumns = append(podColumns, col)
	}

	rootCmd.AddCommand(podsCmd)
	podsCmd.Flags().StringVar(&podSortBy, "sort-by", "name", "Sort pods by: name, cpu-req, cpu-limit, mem-req, mem-limit")
	podsCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "If present, show pods in the specified namespace only")
	podsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show additional columns like NODE")

}

/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	client "github.com/akomic/kubectl-xtop/client"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nodesCmd represents the nodes command
var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Top nodes",
	Run: func(cmd *cobra.Command, args []string) {
		nodePods := make(map[string][]corev1.Pod)
		nodesMeta := make(map[string]map[string]string)
		nodesResources := make(map[string]map[string]*resource.Quantity)

		for {
			nodes, err := client.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}
			// fmt.Printf("There are %d nodes in the cluster\n", len(nodes.Items))

			for _, node := range nodes.Items {
				if _, ok := nodesResources[node.ObjectMeta.Name]; !ok {
					nodesMeta[node.ObjectMeta.Name] = map[string]string{
						"arch": node.ObjectMeta.Labels["kubernetes.io/arch"],
						"os":   node.ObjectMeta.Labels["kubernetes.io/os"],
						"type": node.ObjectMeta.Labels["node.kubernetes.io/instance-type"],
						// "cpuCapacity": node.Status.Capacity.Cpu().String(),
						// "memCapacity": node.Status.Capacity.Memory().String(),
					}
					nodesResources[node.ObjectMeta.Name] = map[string]*resource.Quantity{
						"cpuReq":      &resource.Quantity{},
						"cpuLimit":    &resource.Quantity{},
						"cpuCapacity": node.Status.Capacity.Cpu(),
						"memReq":      &resource.Quantity{},
						"memLimit":    &resource.Quantity{},
						"memCapacity": node.Status.Capacity.Memory(),
					}
				}
			}

			pods, err := client.Clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				panic(err.Error())
			}
			// fmt.Printf("There are %d pods in the cluster\n", len(pods.Items))

			for _, pod := range pods.Items {
				if _, ok := nodePods[pod.Spec.NodeName]; !ok {
					nodePods[pod.Spec.NodeName] = make([]corev1.Pod, 1)
				}
				nodePods[pod.Spec.NodeName] = append(nodePods[pod.Spec.NodeName], pod)
			}

			for nodeName, podList := range nodePods {
				for _, pod := range podList {
					for _, container := range pod.Spec.Containers {
						nodesResources[nodeName]["cpuReq"].Add(*container.Resources.Requests.Cpu())
						nodesResources[nodeName]["cpuLimit"].Add(*container.Resources.Limits.Cpu())
						nodesResources[nodeName]["memReq"].Add(*container.Resources.Requests.Memory())
						nodesResources[nodeName]["memLimit"].Add(*container.Resources.Limits.Memory())
					}
				}
			}

			// w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
			fmt.Fprintln(w, "NAME \t CPU CAPACITY \t CPU REQ \t CPU LIMIT \t MEM CAPACITY \t MEM REQ (%) \t MEM LIMIT")
			for nodeName, data := range nodesResources {
				// buffer := make([]byte, 0, 100)

				cpuReq, cpuReqSuffix := data["cpuReq"].CanonicalizeBytes(make([]byte, 0, 100))
				cpuLimit, cpuLimitSuffix := data["cpuLimit"].CanonicalizeBytes(make([]byte, 0, 100))
				cpuCapacity, cpuCapacitySuffix := data["cpuCapacity"].CanonicalizeBytes(make([]byte, 0, 200))

				memReq, memReqSuffix := data["memReq"].CanonicalizeBytes(make([]byte, 0, 100))
				memLimit, memLimitSuffix := data["memLimit"].CanonicalizeBytes(make([]byte, 0, 100))
				memCapacity, memCapacitySuffix := data["memCapacity"].CanonicalizeBytes(make([]byte, 0, 200))
				fmt.Fprintf(w, "%s \t %s%s \t %s%s (%.2f%s) \t %s%s \t %s%s \t %s%s (%.2f%s) \t %s%s\n", nodeName,
					string(cpuCapacity), cpuCapacitySuffix,
					string(cpuReq), cpuReqSuffix, (float64(data["cpuReq"].Value())/float64(data["cpuCapacity"].Value()))*100, "%",
					string(cpuLimit), cpuLimitSuffix,
					string(memCapacity), memCapacitySuffix,
					string(memReq), memReqSuffix, (float64(data["memReq"].Value())/float64(data["memCapacity"].Value()))*100, "%",
					string(memLimit), memLimitSuffix,
				)
			}
			w.Flush()

			// time.Sleep(10 * time.Second)
			break
		}
	},
}

func init() {
	rootCmd.AddCommand(nodesCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// nodesCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// nodesCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

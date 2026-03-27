package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Cloud deployment management",
	Long:  "Deploy and manage Scout on Kubernetes using Helm.",
}

var cloudDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy Scout to Kubernetes",
	Long:  "Deploy Scout using the bundled Helm chart. Requires helm CLI on PATH.",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace, _ := cmd.Flags().GetString("namespace")
		release, _ := cmd.Flags().GetString("release")
		replicas, _ := cmd.Flags().GetInt("replicas")
		image, _ := cmd.Flags().GetString("image")
		tag, _ := cmd.Flags().GetString("tag")
		valuesFile, _ := cmd.Flags().GetString("values")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		helmArgs := []string{
			"upgrade", "--install", release, "deploy/helm/scout",
			"--namespace", namespace,
			"--create-namespace",
		}

		if replicas > 0 {
			helmArgs = append(helmArgs, "--set", fmt.Sprintf("replicaCount=%d", replicas))
		}
		if image != "" {
			helmArgs = append(helmArgs, "--set", fmt.Sprintf("image.repository=%s", image))
		}
		if tag != "" {
			helmArgs = append(helmArgs, "--set", fmt.Sprintf("image.tag=%s", tag))
		}
		if valuesFile != "" {
			helmArgs = append(helmArgs, "-f", valuesFile)
		}
		if dryRun {
			helmArgs = append(helmArgs, "--dry-run")
		}

		helmBin, err := exec.LookPath("helm")
		if err != nil {
			return fmt.Errorf("scout: helm not found on PATH: %w", err)
		}

		c := exec.Command(helmBin, helmArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		_, _ = fmt.Fprintf(os.Stderr, "Running: helm %s\n", strings.Join(helmArgs, " "))

		return c.Run()
	},
}

var cloudStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace, _ := cmd.Flags().GetString("namespace")
		release, _ := cmd.Flags().GetString("release")
		asJSON, _ := cmd.Flags().GetBool("json")

		helmBin, err := exec.LookPath("helm")
		if err != nil {
			return fmt.Errorf("scout: helm not found on PATH: %w", err)
		}

		statusArgs := []string{"status", release, "--namespace", namespace}
		if asJSON {
			statusArgs = append(statusArgs, "-o", "json")
		}

		c := exec.Command(helmBin, statusArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		return c.Run()
	},
}

var cloudScaleCmd = &cobra.Command{
	Use:   "scale <replicas>",
	Short: "Scale the deployment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace, _ := cmd.Flags().GetString("namespace")
		release, _ := cmd.Flags().GetString("release")

		helmBin, err := exec.LookPath("helm")
		if err != nil {
			return fmt.Errorf("scout: helm not found on PATH: %w", err)
		}

		scaleArgs := []string{
			"upgrade", release, "deploy/helm/scout",
			"--namespace", namespace,
			"--reuse-values",
			"--set", fmt.Sprintf("replicaCount=%s", args[0]),
		}

		c := exec.Command(helmBin, scaleArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		_, _ = fmt.Fprintf(os.Stderr, "Scaling to %s replicas...\n", args[0])

		return c.Run()
	},
}

var cloudUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Scout deployment",
	RunE: func(cmd *cobra.Command, args []string) error {
		namespace, _ := cmd.Flags().GetString("namespace")
		release, _ := cmd.Flags().GetString("release")

		helmBin, err := exec.LookPath("helm")
		if err != nil {
			return fmt.Errorf("scout: helm not found on PATH: %w", err)
		}

		c := exec.Command(helmBin, "uninstall", release, "--namespace", namespace)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		return c.Run()
	},
}

func init() {
	rootCmd.AddCommand(cloudCmd)
	cloudCmd.AddCommand(cloudDeployCmd, cloudStatusCmd, cloudScaleCmd, cloudUninstallCmd)

	// Persistent flags for all cloud commands
	cloudCmd.PersistentFlags().StringP("namespace", "n", "scout", "Kubernetes namespace")
	cloudCmd.PersistentFlags().String("release", "scout", "Helm release name")

	// Deploy flags
	cloudDeployCmd.Flags().Int("replicas", 0, "Number of replicas (0 = use chart default)")
	cloudDeployCmd.Flags().String("image", "", "Container image repository")
	cloudDeployCmd.Flags().String("tag", "", "Container image tag")
	cloudDeployCmd.Flags().StringP("values", "f", "", "Path to values.yaml override file")
	cloudDeployCmd.Flags().Bool("dry-run", false, "Simulate the deployment")

	// Status flags
	cloudStatusCmd.Flags().Bool("json", false, "Output as JSON")
}

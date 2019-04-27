package main

import (
	"flag"
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"github.com/openshift/cluster-bootstrap/pkg/version"
)

var (
	cmdRoot		= &cobra.Command{Use: "cluster-bootstrap", Short: "Bootstrap a control plane!", SilenceErrors: true, Long: ""}
	cmdVersion	= &cobra.Command{Use: "version", Short: "Output version information", Long: "", RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Version: %s\n", version.Version)
		return nil
	}}
)

func main() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	flag.Parse()
	InitLogs()
	defer FlushLogs()
	cmdRoot.AddCommand(cmdVersion)
	if err := cmdRoot.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

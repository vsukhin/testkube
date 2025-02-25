package commands

import (
	"github.com/kubeshop/testkube/pkg/process"
	"github.com/kubeshop/testkube/pkg/ui"
	"github.com/spf13/cobra"
)

func NewUninstallCmd() *cobra.Command {
	var name, namespace string
	var removeCRDs bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Helm chart registry in current kubectl context",
		Long:  `Uninstall Helm chart registry in current kubectl context`,
		Run: func(cmd *cobra.Command, args []string) {

			ui.Verbose = true
			ui.Logo()

			_, err := process.Execute("helm", "uninstall", "--namespace", namespace, name)
			ui.PrintOnError("uninstalling testkube", err)

			if removeCRDs {
				_, err = process.Execute("kubectl", "delete", "crds", "--namespace", namespace, "scripts.tests.testkube.io", "executors.executor.testkube.io")
				ui.PrintOnError("uninstalling CRDs", err)
			}
		},
	}

	cmd.Flags().StringVar(&name, "name", "testkube", "installation name")
	cmd.Flags().StringVar(&namespace, "namespace", "testkube", "namespace where to install")
	cmd.Flags().BoolVar(&removeCRDs, "remove-crds", false, "wipe out Executors and Scripts CRDs")

	return cmd
}

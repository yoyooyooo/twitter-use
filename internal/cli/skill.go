package cli

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillText string

func addSkill(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "skill",
		Short: "Print the agent guide shipped with this binary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), skillText)
			return err
		},
	})
}

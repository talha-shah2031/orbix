package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newVersionCommand(build BuildInfo, output io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, _ := cmd.Root().PersistentFlags().GetString("output")
			if format == "json" {
				return json.NewEncoder(output).Encode(map[string]string{"version": build.Version, "commit": build.Commit, "date": build.Date})
			}
			_, err := fmt.Fprintf(output, "orbix %s (commit %s, built %s)\n", build.Version, build.Commit, build.Date)
			return err
		},
	}
}

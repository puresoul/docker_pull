package cmd

import (
	"go_pull/pkgs/vmconfig"
	"go_pull/pkgs/util/logtool"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "gopull",
		Short: "get a image",
		Long:  `get a image!`,
		TraverseChildren: true,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

func init() {
	logtool.InitEvent(vmconfig.DefaultLoglevel)
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	if err := rootCmd.Execute(); err != nil {
		logtool.SugLog.Fatal(err)
	}
}

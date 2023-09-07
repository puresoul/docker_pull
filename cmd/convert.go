package cmd

import (
	"go_pull/pkgs/config"
	"go_pull/pkgs/util/logtool"
	"go_pull/pkgs/vmconfig"
	"go_pull/pkgs/vmbetter"
	"fmt"
	"github.com/spf13/cobra"
)

var (
	convertCmd = &cobra.Command{
		Use:   "convert",
		Short: "convert image",
		Long:  `convert image!`,
		TraverseChildren: true,
		Run: func(cmd *cobra.Command, args []string) {
			convert(args)
		},
	}
)

func init() {
	rootCmd.AddCommand(convertCmd)
	logtool.InitEvent(config.DefaultLoglevel)
}


func convert(args []string) {
    conf := vmconfig.Config{Path: "tmp"}
    err := vmbetter.BuildDisk("./", conf)
    fmt.Println(err)
}

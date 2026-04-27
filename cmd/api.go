/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cuniBTCReward/api/setup"
	setupconfig "cuniBTCReward/api/setup/config"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

// apiCmd represents the api command
var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("api called")
		var c ServiceConfig
		conf.MustLoad(cfgFile, &c)

		server := rest.MustNewServer(c.ApiConf.RestConf)
		defer server.Stop()

		setupConfig := setupconfig.Config{
			Config: c.ApiConf.Config,
		}
		setup.Setup(server, setupConfig)

		fmt.Printf("Starting server at %s:%d...\n", c.ApiConf.Host, c.ApiConf.Port)
		server.Start()
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// apiCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// apiCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

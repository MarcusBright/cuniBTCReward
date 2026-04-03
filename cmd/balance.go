/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cuniBTCReward/pkg/slack"
	"cuniBTCReward/service/calcamount"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

// balanceCmd represents the balance command
var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("balance called")
		var c ServiceConfig
		conf.MustLoad(cfgFile, &c)
		c.MustSetUp()
		// fmt.Printf("evmscanconfi:%v", c.EvmScanConf)

		//log
		if c.LogSlack != "" {
			logx.AddWriter(logx.NewWriter(slack.NewSlackWriter(c.LogSlack)))
			logx.AddGlobalFields(logx.Field("server", c.CalcConf.Name))
			defer logx.Close()
		}

		calc := calcamount.NewCalulator(&c.CalcConf)
		if block == 0 {
			getBlock, err := calc.GetCursor(chainId)
			if err != nil {
				slack.SendTo(c.CalcConf.NotifySlack, fmt.Sprintf("[%s] %v", c.CalcConf.Name, err))
				fmt.Println(err)
				return
			}
			block = getBlock.BlockNumber
		}
		fmt.Printf("Block: %d, Chain ID: %d, Check Balance: %t\n", block, chainId, checkBalance)
		result, err := calc.GetAllAddressAtBlock(block, chainId, checkBalance)
		if err != nil {
			slack.SendTo(c.CalcConf.NotifySlack, fmt.Sprintf("[%s] %v", c.CalcConf.Name, err))
			fmt.Println(err)
			return
		}
		resultJson, err := json.Marshal(&result)
		if err != nil {
			slack.SendTo(c.CalcConf.NotifySlack, fmt.Sprintf("[%s] %v", c.CalcConf.Name, err))
			fmt.Println(err)
			return
		}
		fmt.Println(string(resultJson))
	},
}

var block uint64
var chainId uint
var checkBalance bool

func init() {
	rootCmd.AddCommand(balanceCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// balanceCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// balanceCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	balanceCmd.PersistentFlags().Uint64VarP(&block, "block", "b", 0, "block number to check balance at, default is latest")
	balanceCmd.PersistentFlags().UintVarP(&chainId, "chainid", "c", 1, "chain ID to check balance on, default is 1 for Ethereum mainnet")
	balanceCmd.PersistentFlags().BoolVarP(&checkBalance, "checkbalance", "k", true, "whether to check token balance")
}

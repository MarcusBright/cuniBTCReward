package config

type Config struct {
	EvmScanConf
}

type EvmScanConf struct {
	Name         string `json:",optional,inherit"`
	DataSource   string `json:",inherit"`
	SqlLog       bool   `json:",optional,default=false,inherit"`
	ChainInfo    []ChainInfo
	LogsScanSpec string
	NotifySlack  string `json:",optional,inherit"`
	LogSlack     string `json:",optional,inherit"`
}

type EvmClient struct {
	ChainId   uint
	ChainName string
	Host      string
	Request   int
	PeriodSec int
}

type ChainInfo struct {
	Client            EvmClient
	CuniBTCVault      string
	DelayRedeemRouter string
}

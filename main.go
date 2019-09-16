package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"crossChainClient/cmd"
	"crossChainClient/common"
	"crossChainClient/config"
	"crossChainClient/log"
	"crossChainClient/service"

	"github.com/joeqian10/neo-utils/neoutils"
	neoRpc "github.com/joeqian10/neo-utils/neoutils/neorpc"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/urfave/cli"
)

func setupApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "Relayer cli"
	app.Action = startSync
	app.Copyright = "Copyright in 2018 The Ontology Authors"
	app.Flags = []cli.Flag{
		cmd.LogLevelFlag,
		cmd.ConfigPathFlag,
	}
	app.Commands = []cli.Command{}
	app.Before = func(context *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		return nil
	}
	return app
}

func main() {
	if err := setupApp().Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startSync(ctx *cli.Context) {
	logLevel := ctx.GlobalInt(cmd.GetFlagName(cmd.LogLevelFlag))
	log.InitLog(logLevel, log.PATH, log.Stdout)
	configPath := ctx.String(cmd.GetFlagName(cmd.ConfigPathFlag))
	err := config.DefConfig.Init(configPath)
	if err != nil {
		fmt.Println("DefConfig.Init error:", err)
		return
	}

	//create Relay Chain RPC Client
	relaySdk := sdk.NewOntologySdk()
	relaySdk.NewRpcClient().SetAddress(config.DefConfig.RelayJsonRpcAddress)

	//Get wallet account from Relay Chain
	account, ok := common.GetAccountByPassword(relaySdk, config.DefConfig.WalletFile)
	if !ok {
		fmt.Println("common.GetAccountByPassword error")
		return
	}

	// create an NEO RPC client
	neoRpcClient := neoRpc.NewClient(config.DefConfig.NeoJsonRpcAddress)
	// create an NEO wallet
	// hard coded the private key here, only for testing
	neoWallet := neoutils.GenerateFromPrivateKey("")

	syncService := service.NewSyncService(account, relaySdk, neoWallet, neoRpcClient)
	syncService.Run()

	waitToExit()
}

func waitToExit() {
	exit := make(chan bool, 0)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for sig := range sc {
			log.Infof("Ontology received exit signal:%v.", sig.String())
			close(exit)
			break
		}
	}()
	<-exit
}

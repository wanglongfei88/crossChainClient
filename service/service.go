package service

import (
	"os"

	"encoding/json"

	"github.com/joeqian10/crossChainClient/config"
	"github.com/joeqian10/crossChainClient/log"

	//vconfig "github.com/ontio/multi-chain/consensus/vbft/config"
	//"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/ont"
	"github.com/joeqian10/neo-utils/neoutils"
	neoRpc "github.com/joeqian10/neo-utils/neoutils/neorpc"
	sdk "github.com/ontio/ontology-go-sdk"
)

type SyncService struct {
	account         *sdk.Account
	relaySdk        *sdk.OntologySdk
	relaySyncHeight uint32

	neoWallet         *neoutils.neoWallet
	neoRpcClient      *neoRpc.NEORPCClient
	neoSyncHeight     uint32
	neoNextConsensus  string
	neoCCMCScriptHash string

	config *config.Config
}

func NewSyncService(acct *sdk.Account, relaySdk *sdk.OntologySdk, neoWallet *neoutils.neoWallet, neoRpcClient *neoRpc.NEORPCClient) *SyncService {
	syncSvr := &SyncService{
		account:      acct,
		relaySdk:     relaySdk,

		neoWallet:    neoWallet,
		neoRpcClient: neoRpcClient,
		config:       config.DefConfig,
	}
	return syncSvr
}

func (this *SyncService) Run() {
	go this.NeoToRelay()
	//go this.RelayToNeo()
}

func (this *SyncService) RelayToNeo() {
	// 侧链上已经同步的主链高度，存放在智能合约header_sync里
	currentNeoChainSyncHeight, err := this.GetCurrentNeoChainSyncHeight(this.GetRelayChainID())
	if err != nil {
		log.Errorf("[RelayToNeo] this.GetCurrentNeoChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.neoSyncHeight = currentNeoChainSyncHeight
	for {
		// 当前主链的高度
		currentMainChainHeight, err := this.relaySdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[RelayToNeo] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.neoSyncHeight; i < currentMainChainHeight; i++ {
			log.Infof("[RelayToNeo] start parse block %d", i)
			//sync key header
			block, err := this.relaySdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[RelayToNeo] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{} // resolve reference issue
			// 把block.Header.ConsensusPayload放到blkInfo里面，
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[RelayToNeo] unmarshal blockInfo error: %s", err)
			}
			// 说明是key header
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToNeo(i)
				if err != nil {
					log.Errorf("[RelayToNeo] this.syncHeaderToNeo error:%s", err)
				}
			}

			//sync cross chain info (transactions)
			// 跨链交易的标记是通过智能合约的event通知来实现的
			events, err := this.relaySdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[RelayToNeo] this.relaySdk.GetSmartContractEventByBlock error:%s", err)
				break
			}
			for _, event := range events {
				for _, notify := range event.Notify {
					states, ok := notify.States.([]interface{}) // notify.States是interface类型的，类似于C#中object
					if !ok {
						continue
					}
					name := states[0].(string)
					// states的数据结构
					if name == ont.MAKE_TO_ONT_PROOF {
						key := states[3].(string)
						err = this.syncHeaderToNeo(i + 1) // 跨链交易发生在n个区块，state root是在n+1个区块里面的
						if err != nil {
							log.Errorf("[RelayToNeo] this.syncHeaderToNeo error:%s", err)
						}
						err := this.syncProofToNeo(key, i)
						if err != nil {
							log.Errorf("[RelayToNeo] this.syncProofToNeo error:%s", err)
						}
					}
				}
			}
			this.neoSyncHeight++
		}
	}
}

func (this *SyncService) NeoToRelay() {
	// 主链上已经同步的侧链高度，存放在主链智能合约header_sync里
	currentRelayChainSyncHeight, err := this.GetCurrentRelayChainSyncHeight(this.GetNeoChainID())
	if err != nil {
		log.Errorf("[NeoToRelay] this.GetCurrentMainChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.relaySyncHeight = currentRelayChainSyncHeight
	this.neoNextConsensus = ""
	for {
		currentNeoChainHeight, err := this.neoSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[NeoToRelay] this.neoSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.relaySyncHeight; i < currentNeoChainHeight; i++ {
			log.Infof("[NeoToRelay] start parse block %d", i)
			//sync key header
			blockResponse := this.neoRpcClient.GetBlockByIndex(i)

			if blockResponse.ErrorResponse != nil {
				log.Errorf("[NeoToRelay] this.neoRpcClient.GetBlockByIndex error:", blockResponse.ErrorResponse.Error.Message)
			}
			// 检查block的nextConsensus是否变化
			// 若变了，就是key header，调用syncHeaderToRelay
			if blockResponse.Result.NextConsensus != this.neoNextConsensus {
				err = this.syncHeaderToRelay(i)
				if err != nil {
					log.Errorf("[NeoToRelay] this.syncHeaderToRelay error:%s", err)
				}
			}

			// sync cross chain info
			// get all transactions from this block i
			txCount := len(blockResponse.Result.Tx)
			// for each transaction txID, call GetApplicationLog
			for i := 0; i < txCount; i++ {
				// 在appLogResponse的notifications字段中的contract字段表示的就是合约脚本哈希
				// 只需要判断该脚本哈希是否和CCMC的脚本哈希一致即可
				appLogResponse := this.neoRpcClient.GetApplicationLog(blockResponse.Result.Tx[i].Txid)
				if appLogResponse.ErrorResponse != nil {
					log.Errorf("[NeoToRelay] this.neoRpcClient.appLogResponse error:", appLogResponse.ErrorResponse.Error.Message)
				}

				for j := 0; j < len(appLogResponse.Result.Executions); j++ {
					for k := 0; k < len(appLogResponse.Result.Executions.Notifications); k++ {
						if appLogResponse.Result.Executions[j].Notifications[k].Contract == this.neoCCMCScriptHash {
							err = this.syncHeaderToRelay(i + 1)
							if err != nil {
								log.Errorf("[NeoToRelay] this.syncHeaderToRelay error:%s", err)
							}
							err := this.syncProofToRelay(key, i)
							if err != nil {
								log.Errorf("[NeoToRelay] this.syncProofToRelay error:%s", err)
							}
						}
					}
				}
			}
			this.relaySyncHeight++
		}
	}

}

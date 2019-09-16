package service

import (
	"os"

	"encoding/json"

	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"

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

	neoWallet	  	*neoutils.neoWallet
	neoRpcClient    *neoRpc.NEORPCClient
	neoSyncHeight   uint32

	config          *config.Config
}

func NewSyncService(acct *sdk.Account, relaySdk *sdk.OntologySdk, neoRpcClient *neoRpc.NEORPCClient) *SyncService {
	syncSvr := &SyncService{
		account:      acct,
		relaySdk:     relaySdk,
		neoRpcClient: neoRpcClient,
		config:       config.DefConfig,
	}
	return syncSvr
}

func (this *SyncService) Run() {
	go this.NeoToAlliance()
	//go this.AllianceToNeo()
}

func (this *SyncService) AllianceToNeo() {
	// 侧链上已经同步的主链高度，存放在智能合约header_sync里
	currentNeoChainSyncHeight, err := this.GetCurrentNeoChainSyncHeight(this.GetRelayChainID())
	if err != nil {
		log.Errorf("[AllianceToNeo] this.GetCurrentNeoChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.neoSyncHeight = currentNeoChainSyncHeight
	for {
		// 当前主链的高度
		currentMainChainHeight, err := this.relaySdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[AllianceToNeo] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.neoSyncHeight; i < currentMainChainHeight; i++ {
			log.Infof("[AllianceToNeo] start parse block %d", i)
			//sync key header
			block, err := this.relaySdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[AllianceToNeo] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{} // resolve reference issue
			// 把block.Header.ConsensusPayload放到blkInfo里面，
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[AllianceToNeo] unmarshal blockInfo error: %s", err)
			}
			// 说明是key header
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToNeo(i)
				if err != nil {
					log.Errorf("[AllianceToNeo] this.syncHeaderToNeo error:%s", err)
				}
			}

			//sync cross chain info
			//sync cross chain info (transactions)
			// 跨链交易的标记是通过智能合约的event通知来实现的
			events, err := this.relaySdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[AllianceToNeo] this.relaySdk.GetSmartContractEventByBlock error:%s", err)
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
							log.Errorf("[AllianceToNeo] this.syncHeaderToNeo error:%s", err)
						}
						err := this.syncProofToNeo(key, i)
						if err != nil {
							log.Errorf("[AllianceToNeo] this.syncProofToNeo error:%s", err)
						}
					}
				}
			}
			this.neoSyncHeight++
		}
	}
}

func (this *SyncService) NeoToAlliance() {
	currentRelayChainSyncHeight, err := this.GetCurrentRelayChainSyncHeight(this.GetNeoChainID())
	if err != nil {
		log.Errorf("[NeoToAlliance] this.GetCurrentMainChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.relaySyncHeight = currentRelayChainSyncHeight
	for {
		currentNeoChainHeight, err := this.neoSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[NeoToAlliance] this.neoSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.relaySyncHeight; i < currentNeoChainHeight; i++ {
			log.Infof("[NeoToAlliance] start parse block %d", i)
			//sync key header
			blockResponse := this.neoRpcClient.GetBlockByIndex(i)
			// 从blockResponse里构造一个block

			// 检查block的nextConsensus有没有变化

			// 若有变化，就是key header，调用syncHeaderToRelay
			if err != nil {
				log.Errorf("[NeoToAlliance] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[NeoToAlliance] unmarshal blockInfo error: %s", err)
			}
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToRelay(i)
				if err != nil {
					log.Errorf("[NeoToAlliance] this.syncHeaderToMain error:%s", err)
				}
			}

			// sync cross chain info
			// get all transactions from this block i

			// for each transaction txID, call GetApplicationLog

			// 在appLogResponse的notifications字段中的contract字段表示的就是合约脚本哈希

			// 只需要判断该脚本哈希是否和CCMC的脚本哈希一致即可
			appLogResponse := this.neoRpcClient.GetApplicationLog()


			events, err := this.neoSdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[NeoToAlliance] this.neoSdk.GetSmartContractEventByBlock error:%s", err)
				break
			}
			for _, event := range events {
				for _, notify := range event.Notify {
					states, ok := notify.States.([]interface{})
					if !ok {
						continue
					}
					name := states[0].(string)
					if name == ont.MAKE_FROM_ONT_PROOF {
						key := states[3].(string)
						err = this.syncHeaderToRelay(i + 1)
						if err != nil {
							log.Errorf("[NeoToAlliance] this.syncHeaderToRelay error:%s", err)
						}
						err := this.syncProofToRelay(key, i)
						if err != nil {
							log.Errorf("[NeoToAlliance] this.syncProofToRelay error:%s", err)
						}
					}
				}
			}
			this.relaySyncHeight++
		}
	}

}

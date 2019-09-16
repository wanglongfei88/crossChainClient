package service

import (
	"fmt"
	"time"
	"math"

	"github.com/joeqian10/crossChainClient/common"
	"github.com/joeqian10/crossChainClient/log"

	//"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/inf"

	//"github.com/ontio/ontology/smartcontract/service/native/header_sync"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
	"github.com/siovanus/ontology/smartcontract/service/native/header_sync"

	"github.com/joeqian10/neo-utils/neoutils"
	"github.com/joeqian10/neo-utils/neoutils/smartcontract"
	"github.com/joeqian10/neo-utils/neoutils/tx"
)

var codeVersion = byte(0)

func (this *SyncService) GetRelayChainID() uint64 {
	return this.config.RelayChainID
}

func (this *SyncService) GetNeoChainID() uint64 {
	return this.config.NeoChainID
}

func (this *SyncService) GetGasPrice() uint64 {
	return this.config.GasPrice
}

func (this *SyncService) GetGasLimit() uint64 {
	return this.config.GasLimit
}

// GetCurrentNeoChainSyncHeight 在侧链的智能合约存储区得到已同步过来的中继链高度
func (this *SyncService) GetCurrentNeoChainSyncHeight(relayChainID uint64) (uint32, error) {
	contractAddress := utils.HeaderSyncContractAddress // can be hard coded

	relayChainIDBytes, err := utils.GetUint64Bytes(relayChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}

	// "currentHeight"+"2"
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), relayChainIDBytes)

	value, err := this.neoSdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height, err := utils.GetBytesUint32(value)
	if err != nil {
		return 0, fmt.Errorf("GetBytesUint32, get height error: %s", err)
	}
	return height, nil
}

// GetCurrentRelayChainSyncHeight 在中继链的智能合约存储区得到已同步过来的侧链高度
func (this *SyncService) GetCurrentRelayChainSyncHeight(neoChainID uint64) (uint32, error) {
	contractAddress := utils.HeaderSyncContractAddress
	neoChainIDBytes, err := utils.GetUint64Bytes(neoChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), neoChainIDBytes)
	value, err := this.relaySdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height, err := utils.GetBytesUint32(value)
	if err != nil {
		return 0, fmt.Errorf("GetBytesUint32, get height error: %s", err)
	}
	return height, nil
}

func (this *SyncService) syncHeaderToRelay(height uint32) error {
	chainIDBytes, err := utils.GetUint64Bytes(this.GetNeoChainID())
	if err != nil {
		return fmt.Errorf("[syncHeaderToRelay] chainIDBytes, getUint32Bytes error: %v", err)
	}
	heightBytes, err := utils.GetUint32Bytes(height)
	if err != nil {
		return fmt.Errorf("[syncHeaderToRelay] heightBytes, getUint32Bytes error: %v", err)
	}
	v, err := this.relaySdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if len(v) != 0 {
		return nil
	}
	contractAddress := utils.HeaderSyncContractAddress
	method := header_sync.SYNC_BLOCK_HEADER
	block, err := this.neoSdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToRelay] this.mainSdk.GetBlockByHeight error:%s", err)
	}
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{block.Header.ToArray()},
	}
	txHash, err := this.relaySdk.Native.InvokeNativeContract(this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncHeaderToRelay] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncHeaderToRelay] syncHeaderToRelay txHash is :", txHash.ToHexString())
	this.waitForRelayBlock()
	return nil
}

/* func (this *SyncService) syncProofToRelay(key string, height uint32) error {
	TODO: filter if tx is done

	k, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("[syncProofToRelay] hex.DecodeString error: %s", err)
	}
	proof, err := this.neoSdk.GetCrossStatesProof(height, k)
	if err != nil {
		return fmt.Errorf("[syncProofToRelay] this.neoSdk.GetCrossStatesProof error: %s", err)
	}

	contractAddress, _ := ocommon.AddressParseFromBytes([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10})
	method := "ImportOuterTransfer"
	param := &inf.EntranceParam{
		SourceChainID:  this.GetNeoChainID(),
		Height:         height + 1,
		Proof:          proof.AuditPath,
		RelayerAddress: this.account.Address.ToBase58(),
		TargetChainID:  this.GetRelayChainID(),
	}
	txHash, err := this.relaySdk.Native.InvokeNativeContract(this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncProofToRelay] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncProofToRelay] syncProofToRelay txHash is :", txHash.ToHexString())
	return nil
} */

func (this *SyncService) syncHeaderToNeo(height uint32) error {
	chainIDBytes, err := utils.GetUint64Bytes(this.GetRelayChainID())
	if err != nil {
		return fmt.Errorf("[syncHeaderToNeo] chainIDBytes, getUint32Bytes error: %v", err)
	}
	heightBytes, err := utils.GetUint32Bytes(height)
	if err != nil {
		return fmt.Errorf("[syncHeaderToNeo] heightBytes, getUint32Bytes error: %v", err)
	}

	// 从header_sync合约里去拿对应height高度的值，如果返回值的长度不为0，说明已经同步过了
	neoHeaderSyncContractScriptHash := "TBD"
	headerIndex := "headerIndex"
	r := this.neoRpcClient.GetStorage(neoHeaderSyncContractScriptHash,
		common.ConcatKey([]byte(headerIndex), chainIDBytes, heightBytes))
	// r.Result is a string
	if len(r.Result) != 0 {
		return nil
	}

	contractAddress := utils.HeaderSyncContractAddress // can be hard coded
	syncBlockHeader := "syncBlockHeader"               // method, can be hard coded
	block, err := this.relaySdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToNeo] this.mainSdk.GetBlockByHeight error:%s", err)
	}

	// header_sync.SyncBlockHeaderParam待定
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{block.Header.ToArray()},
	}

	// example: SendRawTransaction来构造一笔InvocationTransaction
	// create an InvocationTransaction
	itx := tx.CreateInvocationTransaction()
	// build script
	scriptBuilder := smartcontract.NewScriptBuilder()
	script := scriptBuilder.GenerateContractInvocationScript(neoHeaderSyncContractScriptHash, syncBlockHeader, []interface{ param })
	// invoke script to get gas consumed
	response := this.neoRpcClient.InvokeScript(bytesToHex(script))
	gasString := ConvertString(response.Result.GasConsumed)
	// fulfill the transaction
	itx.ExtraData.Script = script
	itx.ExtraData.ScriptLength = len(script)
	itx.ExtraData.GasConsumed = uint64(gasString)
	// sign the transaction
	unsignedRaw := itx.UnsignedRawTransaction()
	signedData, err := Sign(unsignedRaw, bytesToHex(neoWallet.PrivateKey))
	if (err != nil) {
		return fmt.Errorf("[syncHeaderToNeo] Sign error: %s", err)
	}
	signature := smartcontract.TransactionSignature{
		SignedData: signedData,
		PublicKey:  neoWallet.PublicKey,
	}
	scripts := []interface{}{signature}
	txScripts := smartcontract.NewScriptBuilder().GenerateVerificationScripts(scripts)
	itx.witness = txScripts
	// send the raw transaction
	response := this.neoRpcClient.SendRawTransaction(tx.RawTransactionString())
	if (response.Result != true) {
		return fmt.Errorf("[syncHeaderToNeo] SendRawTransaction error: %s", response.ErrorResponse.Message)
	}

	log.Infof("[syncHeaderToNeo] syncHeaderToNeo txHash is :", itx.TXID())
	this.waitForNeoBlock()
	return nil
}

/* func (this *SyncService) syncProofToNeo(key string, height uint32) error {
	//TODO: filter if tx is done

	k, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("[syncProofToNeo] hex.DecodeString error: %s", err)
	}
	proof, err := this.relaySdk.GetCrossStatesProof(height, k)
	if err != nil {
		return fmt.Errorf("[syncProofToNeo] this.neoSdk.GetMptProof error: %s", err)
	}

	crossChainAddress, _ := ocommon.AddressParseFromBytes([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08})
	contractAddress := crossChainAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		Address:     this.account.Address,
		FromChainID: this.GetRelayChainID(),
		Height:      height + 1,
		Proof:       proof.AuditPath,
	}
	txHash, err := this.neoSdk.Native.InvokeNativeContract(this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncProofToNeo] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncProofToNeo] syncProofToNeo txHash is :", txHash.ToHexString())
	return nil
} */

func (this *SyncService) waitForRelayBlock() {
	_, err := this.relaySdk.WaitForGenerateBlock(90*time.Second, 3)
	if err != nil {
		log.Errorf("waitForRelayBlock error:%s", err)
	}
}

func (this *SyncService) waitForNeoBlock() {
	time.Sleep(time.Duration(15)*time.Second)
	/* _, err := this.neoSdk.WaitForGenerateBlock(90*time.Second, 3)
	if err != nil {
		log.Errorf("waitForNeoBlock error:%s", err)
	} */
}

package common

import (
	"fmt"

	"github.com/ontio/multi-chain/common/password"
	sdk "github.com/ontio/ontology-go-sdk"
)

func GetAccountByPassword(sdk *sdk.OntologySdk, path string) (*sdk.Account, bool) {
	wallet, err := sdk.OpenWallet(path)
	if err != nil {
		fmt.Println("open wallet error:", err)
		return nil, false
	}
	pwd, err := password.GetPassword()
	if err != nil {
		fmt.Println("getPassword error:", err)
		return nil, false
	}
	user, err := wallet.GetDefaultAccount(pwd)
	if err != nil {
		fmt.Println("getDefaultAccount error:", err)
		return nil, false
	}
	return user, true
}

func ConcatKey(args ...[]byte) []byte {
	temp := []byte{}
	for _, arg := range args {
		temp = append(temp, arg...)
	}
	return temp
}

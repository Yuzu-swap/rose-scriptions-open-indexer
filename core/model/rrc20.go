package model

import (
	"errors"
	"fmt"
	"math/big"
	"rose-scriptions-open-indexer/utils/generics/must"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type RRC20Operation string
type ValideCode int8
type RRC20OrderStatus int

var (
	RRC20ProtocolName = "rrc-20"
)

const (
	RRC20OperationTransfer RRC20Operation = "transfer"
	RRC20OperationDeploy   RRC20Operation = "deploy"
	RRC20OperationMint     RRC20Operation = "mint"
	RRC20OperationList     RRC20Operation = "list"
	RRC20OperationExchange RRC20Operation = "exchange"

	ValidCodeUnknowError     ValideCode = 0
	ValidCodeOK              ValideCode = 1
	ValidCodeEmptyTick       ValideCode = -1
	ValidCodeTooLongTick     ValideCode = -2
	ValideCodeWrongOperation ValideCode = -3

	ValidCodeWrongMax        ValideCode = -11
	ValideCodeWrongPrecision ValideCode = -12
	ValidCodeLimitNotExists  ValideCode = -13
	ValidCodeWrongMaxLimit   ValideCode = -14
	ValidCodeInvalidSign     ValideCode = -15
	ValidCodeOverLimit       ValideCode = -16
	ValidCodeTokenDeployed   ValideCode = -17

	ValidCodeAmountNotExists    ValideCode = -21
	ValidCodeAmountError        ValideCode = -22
	ValidCodeTokenNotExists     ValideCode = -23
	ValideCodePrecisionNotEqual ValideCode = -24
	ValidCodeOverTotalLimit     ValideCode = -27

	ValidCodeTransferToSelf      ValideCode = -28
	ValidCodeBalanceNotSatisfied ValideCode = -29

	ValidCodeListToSelf                ValideCode = -30
	ValidCodeListIdNotExists           ValideCode = -31
	ValidCodeListHasTransferd          ValideCode = -32
	ValidCodeListOriginAddressNotMatch ValideCode = -33
	ValidCodeListAddressNotMatch       ValideCode = -34
)

var (
	DocumentNotExists = errors.New("document not exists")
)

type RRC20 struct {
	Number    uint64         //global inscription Number
	Hash      string         `gorm:"index:idx_hash,unique"`
	Tick      string         `gorm:"index:idx_tick_from,index:index_tick_to,index:idx_tick_oper"`
	Operation RRC20Operation `gorm:"index:idx_tick_oper"`
	// deploy args
	From      string `gorm:"index:idx_tick_from"`
	To        string `gorm:"index:index_tick_to"`
	Precision int
	Max       *DDecimal
	Limit     *DDecimal
	Timestamp uint64
	// transfer and mint args
	Amount *DDecimal
	Valid  ValideCode
}

func (code ValideCode) String() string {
	messages := map[ValideCode]string{
		ValidCodeUnknowError:         "Unknown error",
		ValidCodeOK:                  "Operation successful",
		ValidCodeEmptyTick:           "Empty tick",
		ValidCodeTooLongTick:         "Tick is too long",
		ValideCodeWrongOperation:     "Wrong operation",
		ValidCodeWrongMax:            "Wrong max value",
		ValideCodeWrongPrecision:     "Wrong precision",
		ValidCodeLimitNotExists:      "Limit does not exist",
		ValidCodeWrongMaxLimit:       "Wrong max limit",
		ValidCodeInvalidSign:         "Invalid sign",
		ValidCodeOverLimit:           "Over limit",
		ValidCodeTokenDeployed:       "Token already deployed",
		ValidCodeAmountNotExists:     "Amount does not exist",
		ValidCodeAmountError:         "Amount error",
		ValidCodeTokenNotExists:      "Token does not exist",
		ValideCodePrecisionNotEqual:  "Precision not equal",
		ValidCodeOverTotalLimit:      "Over total limit",
		ValidCodeTransferToSelf:      "Cannot transfer to self",
		ValidCodeBalanceNotSatisfied: "Balance not satisfied",
	}

	msg, ok := messages[code]
	if !ok {
		return "Unrecognized error code"
	}
	return msg
}

type RRCListedEvent struct {
	From common.Address
	To   common.Address
	Id   [32]byte
}

func (rrc *RRCListedEvent) Hash() string {
	return fmt.Sprintf("0x%x", rrc.Id)
}

type RRCOrderExecutedEvent struct {
	Seller    common.Address
	Taker     common.Address
	ListId    [32]byte
	Ticker    string
	Amount    *big.Int
	Price     *big.Int
	FeeRate   uint16
	Timestamp uint64
}

func (rrc *RRCOrderExecutedEvent) Hash() string {
	return fmt.Sprintf("0x%x", rrc.ListId)
}

type RRCOrderCanceledEvent struct {
	Seller    common.Address
	ListId    [32]byte
	Timestamp uint64
}

func (rrc *RRCOrderCanceledEvent) Hash() string {
	return fmt.Sprintf("0x%x", rrc.ListId)
}

const RRCEventABIJson = `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"from","type":"address"},{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"bytes32","name":"id","type":"bytes32"}],"name":"rosescriptions_protocol_TransferRRC20TokenForListing","type":"event"}]`

var (
	RRCEventABI = must.Must(abi.JSON(strings.NewReader(RRCEventABIJson)))

	TopicsRRCTransferForListing = "0x" + Keccak256("rosescriptions_protocol_TransferRRC20TokenForListing(address,address,bytes32)")
	RRCListEventName            = "rosescriptions_protocol_TransferRRC20TokenForListing"
)

func ParseEventLog(parsedAbi abi.ABI, eventName string, logData *types.Log) (map[string]interface{}, error) {
	event, exists := parsedAbi.Events[eventName]
	if !exists {
		return nil, fmt.Errorf("event '%s' not found", eventName)
	}

	var err error
	eventData := make(map[string]interface{})
	err = parsedAbi.UnpackIntoMap(eventData, eventName, logData.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack event data: %w", err)
	}

	for i, topic := range logData.Topics[1:] {
		indexedName := event.Inputs[i].Name
		eventData[indexedName] = topic
	}

	return eventData, nil
}

func ParseListEvent(parsedAbi abi.ABI, logData *types.Log) (*RRCListedEvent, error) {
	eventData, err := ParseEventLog(parsedAbi, RRCListEventName, logData)
	if err != nil {
		return nil, err
	}

	var from, to common.Address
	var id [32]byte

	if _from, ok := eventData["from"].(common.Hash); ok {
		from = common.BytesToAddress(_from[:])
	}

	if _to, ok := eventData["to"].(common.Hash); ok {
		to = common.BytesToAddress(_to[:])
	}

	if _id, ok := eventData["id"].([32]byte); ok {
		id = _id
	}

	return &RRCListedEvent{
		From: from,
		To:   to,
		Id:   id,
	}, nil
}

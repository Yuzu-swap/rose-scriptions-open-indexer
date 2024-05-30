package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"rose-scriptions-open-indexer/core/model"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
)

var (
	ErrorNoTransaction = errors.New("no transaction")
	ErrorNoPrefix      = errors.New("no prefix")
	ErrorDecode        = errors.New("decode error")

	LatestBlockNumber uint64 = 10320518

	inscriptionNumber uint64 = 0
	rrc20Records      []*model.RRC20
	tokens            = make(map[string]*model.Token)
	tokenHolders      = make(map[string]map[string]*model.DDecimal)
	balances          = make(map[string]map[string]*model.DDecimal)
	lists             = make(map[string]*model.ListedRecord)
)

var mintLimitWhiteList map[string]bool = map[string]bool{
	"0xf9f128d9b8ddb66883708ba08a171e9018bed559": true,
}

func HandleNewBlock(block *model.ChainBlock) error {
	logrus.Infof("handle block %d", block.Number)

	if LatestBlockNumber != block.Number-1 {
		logrus.Warn("block number not match, latest: ", LatestBlockNumber, ", current: ", block.Number)
		return errors.New("block number not match")
	}

	for _, trx := range block.Txs {
		code, err := handleTransaction(trx)
		if err != nil {
			if code != 0 {
				return err
			}
		}
	}

	for _, receipt := range block.Receipts {
		code, err := handleReceipt(receipt)
		if err != nil {
			if code != 0 {
				return err
			}
		}
	}

	LatestBlockNumber++

	return nil
}

func handleTransaction(trx *model.ChainTransaction) (int, error) {
	// data:,
	if !strings.HasPrefix(trx.Input, "0x646174613a") { //data:
		return 0, ErrorNoPrefix
	}
	logrus.Infof("transaction input start with 0x646174613a, input %s", trx.Input)
	// trim 0x
	bytes, err := hex.DecodeString(trx.Input[2:])
	if err != nil {
		logrus.Warn("inscribe err", err, " at block ", trx.Block, ":", trx.Idx)
		return 0, ErrorDecode
	}
	input := string(bytes)

	sepIdx := strings.Index(input, ",")
	if sepIdx == -1 || sepIdx == len(input)-1 {
		return 0, errors.New("no content")
	}
	contentType := "text/plain"
	if sepIdx > 5 {
		contentType = input[5:sepIdx]
	}
	content := input[sepIdx+1:]

	newInscriptionNumber := inscriptionNumber

	if !utf8.ValidString(content) {
		logrus.Infof("content %v is not valid utf8 string", content)
		return 0, errors.New("content is not valid utf8 string")
	}

	var inscription model.Inscription
	inscription.Number = newInscriptionNumber
	inscription.Hash = trx.Id
	inscription.From = trx.From
	inscription.To = trx.To
	inscription.Block = trx.Block
	inscription.Idx = trx.Idx
	inscription.Timestamp = trx.Timestamp
	inscription.ContentType = contentType
	inscription.Content = content

	if code, err := handleProtocols(&inscription); err != nil {
		logrus.Info("error at ", inscription.Number)

		return code, err
	}

	inscriptionNumber++

	return 0, nil
}

func handleProtocols(inscription *model.Inscription) (int, error) {
	content := strings.TrimSpace(inscription.Content)
	logrus.Infof("HandleProtocol: %v,content %v ", inscription, content)
	if content[0] == '{' {
		var protoData map[string]string
		var rawProtoData map[string]interface{}
		err := json.Unmarshal([]byte(content), &rawProtoData)
		if err != nil {
			logrus.Info("json parse error: ", err, ", at ", inscription.Number)
		} else {
			protoData = make(map[string]string)
			for k, v := range rawProtoData {
				if vstr, ok := v.(string); ok {
					protoData[k] = vstr
				}
			}

			value, ok := protoData["p"]
			if ok && strings.TrimSpace(value) != "" {
				protocol := strings.ToLower(value)
				if protocol == model.RRC20ProtocolName {
					var rrc20 model.RRC20
					rrc20.Number = inscription.Number
					rrc20.Hash = inscription.Hash
					if value, ok = protoData["tick"]; ok {
						rrc20.Tick = value
					}
					if value, ok = protoData["op"]; ok {
						rrc20.Operation = model.RRC20Operation(value)
					}

					rrc20.From = strings.ToLower(inscription.From)
					rrc20.To = strings.ToLower(inscription.To)
					rrc20.Timestamp = inscription.Timestamp

					logrus.Infof("protocol: %v", protoData)
					var err error
					if strings.TrimSpace(rrc20.Tick) == "" {
						rrc20.Valid = -1 // empty tick
					} else if len(rrc20.Tick) > 18 {
						rrc20.Valid = -2 // too long tick
					} else if rrc20.Operation == model.RRC20OperationDeploy {
						rrc20.Valid, err = deployToken(&rrc20, inscription, protoData)
						if rrc20.Valid != model.ValidCodeOK {
							logrus.Warnf("deploy token error: %s", rrc20.Valid)
						}
					} else if rrc20.Operation == model.RRC20OperationMint {
						rrc20.Valid, err = mintToken(&rrc20, inscription, protoData)
						if rrc20.Valid != model.ValidCodeOK {
							logrus.Warnf("mint token error: %s", rrc20.Valid)
						}
					} else if rrc20.Operation == model.RRC20OperationTransfer {
						rrc20.Valid, err = transferToken(&rrc20, inscription, protoData)
						if rrc20.Valid != model.ValidCodeOK {
							logrus.Warnf("transfer token error: %s", rrc20.Valid)
						}
					} else if rrc20.Operation == model.RRC20OperationList {
						rrc20.Valid, err = listToken(&rrc20, inscription, protoData)
						if rrc20.Valid != model.ValidCodeOK {
							logrus.Warnf("list token error: %s", rrc20.Valid)
						}
					} else {
						rrc20.Valid = -3 // wrong operation
					}

					if err != nil {
						return -1, err
					}

					rrc20Records = append(rrc20Records, &rrc20)

					return 0, nil
				}
			}
		}
	}
	return 0, nil
}

func deployToken(rrc20 *model.RRC20, inscription *model.Inscription, params map[string]string) (model.ValideCode, error) {

	logrus.Infof("HandleProtocol deploy token: %v,inscription %v", params, inscription)
	value, ok := params["max"]
	if !ok {
		return model.ValidCodeWrongMax, nil
	}
	max, precision, err1 := model.NewDecimalFromString(value)
	if err1 != nil {
		return model.ValideCodeWrongPrecision, nil
	}
	value, ok = params["lim"]
	if !ok {
		return model.ValidCodeLimitNotExists, nil
	}
	limit, _, err2 := model.NewDecimalFromString(value)
	if err2 != nil {
		return model.ValidCodeWrongMaxLimit, nil
	}
	if max.Sign() <= 0 || limit.Sign() <= 0 {
		return model.ValidCodeInvalidSign, nil
	}
	if max.Cmp(limit) < 0 {
		return model.ValidCodeOverLimit, nil
	}

	rrc20.Max = max
	rrc20.Precision = precision
	rrc20.Limit = limit

	rrc20.Tick = strings.TrimSpace(rrc20.Tick)
	lowerTick := strings.ToLower(rrc20.Tick)
	_, exists := tokens[lowerTick]
	if exists {
		return -17, nil
	}

	token := &model.Token{
		Tick:          rrc20.Tick,
		Number:        rrc20.Number,
		Precision:     precision,
		Max:           max,
		Limit:         limit,
		Minted:        model.NewDecimal(),
		Progress:      0,
		CreatedAt:     inscription.Timestamp,
		CompletedAt:   int64(0),
		DeployAddress: inscription.To,
		DeployHash:    inscription.Hash,
	}

	// save
	tokens[lowerTick] = token
	tokenHolders[lowerTick] = make(map[string]*model.DDecimal)

	return 1, nil
}

func mintToken(rrc20 *model.RRC20, inscription *model.Inscription, params map[string]string) (model.ValideCode, error) {
	logrus.Infof("HandleProtocol mint token: %v,inscription %v", params, inscription)
	value, ok := params["amt"]
	if !ok {
		return model.ValidCodeAmountNotExists, nil
	}
	amt, precision, err := model.NewDecimalFromString(value)
	if err != nil {
		return model.ValidCodeAmountError, nil
	}

	rrc20.Amount = amt

	lowerTick := strings.ToLower(rrc20.Tick)
	token, exists := tokens[lowerTick]
	if !exists {
		return model.ValidCodeTokenNotExists, nil
	}

	// check precision
	if precision > token.Precision {
		return model.ValideCodePrecisionNotEqual, nil
	}

	if amt.Sign() <= 0 {
		return model.ValidCodeInvalidSign, nil
	}

	logrus.Infof("token: %v,amt %v ,limit %v ", token, amt, token.Limit)

	_, findFrom := mintLimitWhiteList[strings.ToLower(inscription.From)]
	_, findTo := mintLimitWhiteList[strings.ToLower(inscription.To)]
	if !findFrom || !findTo {
		if amt.Cmp(token.Limit) == 1 {
			return model.ValidCodeWrongMaxLimit, nil
		}
	}

	var left = token.Max.Sub(token.Minted)

	if left.Cmp(amt) == -1 {
		if left.Sign() > 0 {
			amt = left
		} else {
			// exceed max
			return model.ValidCodeOverTotalLimit, nil
		}
	}
	// update amount
	rrc20.Amount = amt

	newHolder, err := addBalance(rrc20.To, lowerTick, amt)
	if err != nil {
		return 0, err
	}

	// update token
	token.Minted = token.Minted.Add(amt)
	token.Trxs++

	if token.Minted.Cmp(token.Max) >= 0 {
		token.Progress = 1000000
	} else {
		progress, _ := new(big.Int).SetString(token.Minted.String(), 10)
		max, _ := new(big.Int).SetString(token.Max.String(), 10)
		progress.Mul(progress, new(big.Int).SetInt64(1000000))
		progress.Div(progress, max)
		token.Progress = int32(progress.Int64())
	}

	if token.Minted.Cmp(token.Max) == 0 {
		token.CompletedAt = int64(time.Now().Unix())
	}
	if newHolder {
		token.Holders++
	}

	return 1, err
}

func transferToken(rrc20 *model.RRC20, inscription *model.Inscription, params map[string]string) (model.ValideCode, error) {
	logrus.Infof("Handle Protocol transfer token: %v,inscription %v", params, inscription)
	value, ok := params["amt"]
	if !ok {
		return model.ValidCodeAmountNotExists, nil
	}
	amt, precision, err := model.NewDecimalFromString(value)
	if err != nil {
		return model.ValidCodeAmountError, nil
	}

	// check token
	lowerTick := strings.ToLower(rrc20.Tick)
	token, exists := tokens[lowerTick]
	if !exists {
		return model.ValidCodeTokenNotExists, nil
	}

	// check precision
	if precision > token.Precision {
		return model.ValideCodePrecisionNotEqual, nil
	}

	if amt.Sign() <= 0 {
		return model.ValidCodeInvalidSign, nil
	}

	if inscription.From == inscription.To {
		// send to self
		return model.ValidCodeTransferToSelf, nil
	}

	rrc20.Amount = amt

	// From
	reduceHolder, err := subBalance(rrc20.From, lowerTick, rrc20.Amount)
	if err != nil {
		if err.Error() == "insufficient balance" {
			return model.ValidCodeBalanceNotSatisfied, nil
		}
		return model.ValidCodeUnknowError, err
	}

	// To
	newHolder, err := addBalance(rrc20.To, lowerTick, rrc20.Amount)
	if err != nil {
		return model.ValidCodeUnknowError, err
	}

	// update token
	if reduceHolder {
		token.Holders--
	}
	if newHolder {
		token.Holders++
	}
	token.Trxs++

	return model.ValidCodeOK, err
}

func listToken(rrc20 *model.RRC20, inscription *model.Inscription, params map[string]string) (model.ValideCode, error) {
	logrus.Infof("Handle Protocol list token: %v,inscription %v", params, inscription)
	value, ok := params["amt"]
	if !ok {
		return model.ValidCodeAmountNotExists, nil
	}
	amt, precision, err := model.NewDecimalFromString(value)
	if err != nil {
		return model.ValidCodeAmountError, nil
	}

	// check token
	lowerTick := strings.ToLower(rrc20.Tick)
	token, exists := tokens[lowerTick]
	if !exists {
		return model.ValidCodeTokenNotExists, nil
	}

	// check precision
	if precision > token.Precision {
		logrus.Errorf("listToken ValideCodePrecisionNotEqual")
		return model.ValideCodePrecisionNotEqual, nil
	}

	if amt.Sign() <= 0 {
		logrus.Errorf("listToken ValidCodeInvalidSign")
		return model.ValidCodeInvalidSign, nil
	}

	if inscription.From == inscription.To {
		// send to self
		logrus.Errorf("listToken ValidCodeListToSelf")
		return model.ValidCodeListToSelf, nil
	}

	rrc20.Amount = amt

	// sub balance
	reduceHolder, err := subBalance(rrc20.From, lowerTick, rrc20.Amount)
	if err != nil {
		if err.Error() == "insufficient balance" {
			return -37, nil
		}
		return 0, err
	}

	// insert list record
	listRec := &model.ListedRecord{
		Hash:       inscription.Hash,
		Tick:       rrc20.Tick,
		OriginAddr: strings.ToLower(inscription.From),
		ListedTo:   strings.ToLower(inscription.To),
		Amount:     amt,
		ListedTs:   inscription.Timestamp,
	}
	lists[listRec.Hash] = listRec

	if reduceHolder {
		token.Holders--
	}

	return 1, err
}

func handleReceipt(receipt *model.ChainReceipt) (int, error) {
	for _, log := range receipt.Logs {
		if log.Topics[0].Hex() == model.TopicsRRCTransferForListing {
			event, err := model.ParseListEvent(model.RRCEventABI, log)
			if err != nil {
				logrus.Warnf("unpack event %s error: %s", model.RRCListEventName, err)
				continue
			}

			eventStr, _ := json.Marshal(event)
			logrus.Infof("handleReceipt hash:%s eventName: %s event: %s", receipt.TxHash.Hex(), model.RRCListEventName, eventStr)

			if _, err := handleRRCListEvent(receipt.TxHash.Hex(), log.Address, event, receipt.Timestamp); err != nil {
				return -1, err
			}
		}
	}

	return 0, nil
}

func handleRRCListEvent(txHash string, logAddress common.Address, event *model.RRCListedEvent, timestamp uint64) (model.ValideCode, error) {
	var rrc20 model.RRC20 = model.RRC20{
		Number:    0,
		Hash:      txHash,
		Operation: model.RRC20OperationExchange,
		From:      event.From.Hex(),
		To:        event.To.Hex(),
		Timestamp: timestamp,
		Valid:     model.ValidCodeOK,
	}

	listRec, ok := lists[event.Hash()]
	if ok {
		// check token
		lowerTick := strings.ToLower(listRec.Tick)
		token, _ := tokens[lowerTick]

		rrc20.Tick = listRec.Tick
		rrc20.Precision = token.Precision
		rrc20.Max = token.Max
		rrc20.Limit = token.Limit
		rrc20.Amount = listRec.Amount

		if listRec.OriginAddr == strings.ToLower(event.From.Hex()) && listRec.ListedTo == strings.ToLower(logAddress.Hex()) {
			// add balance
			newHolder, err := addBalance(event.To.Hex(), listRec.Tick, listRec.Amount)
			if err != nil {
				return model.ValidCodeUnknowError, err
			}

			token.Trxs++

			if newHolder {
				token.Holders++
			}

			delete(lists, listRec.Hash)
		} else {
			if listRec.OriginAddr == strings.ToLower(event.From.Hex()) {
				rrc20.Valid = model.ValidCodeListOriginAddressNotMatch
			} else {
				rrc20.Valid = model.ValidCodeListAddressNotMatch
			}
		}

	} else {
		logrus.Errorf("query list record %s error", event.Hash())
		rrc20.Valid = model.ValidCodeListIdNotExists
	}

	rrc20Records = append(rrc20Records, &rrc20)

	return model.ValidCodeOK, nil
}

func subBalance(owner string, tick string, amount *model.DDecimal) (bool, error) {
	lowerTick := strings.ToLower(tick)
	_, exists := tokens[lowerTick]
	if !exists {
		return false, errors.New("token not found")
	}
	fromBalance, ok := tokenHolders[lowerTick][owner]
	if !ok || fromBalance.Sign() == 0 || amount.Cmp(fromBalance) == 1 {
		return false, errors.New("insufficient balance")
	}

	fromBalance = fromBalance.Sub(amount)

	var reduceHolder = false
	if fromBalance.Sign() == 0 {
		reduceHolder = true
	}

	// save
	tokenHolders[lowerTick][owner] = fromBalance

	if _, ok := balances[owner]; !ok {
		balances[owner] = make(map[string]*model.DDecimal)
	}
	balances[owner][lowerTick] = fromBalance

	return reduceHolder, nil
}

func addBalance(owner string, tick string, amount *model.DDecimal) (bool, error) {
	lowerTick := strings.ToLower(tick)
	_, exists := tokens[lowerTick]
	if !exists {
		return false, errors.New("token not found")
	}
	var newHolder = false
	toBalance, ok := tokenHolders[lowerTick][owner]
	if !ok {
		toBalance = model.NewDecimal()
		newHolder = true
	}

	toBalance = toBalance.Add(amount)

	// save
	tokenHolders[lowerTick][owner] = toBalance

	if _, ok := balances[owner]; !ok {
		balances[owner] = make(map[string]*model.DDecimal)
	}
	balances[owner][lowerTick] = toBalance

	return newHolder, nil
}

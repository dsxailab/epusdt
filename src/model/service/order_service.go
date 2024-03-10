package service

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	. "github.com/ahmetb/go-linq/v3"
	"github.com/assimon/luuu/config"
	"github.com/assimon/luuu/model/dao"
	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/mdb"
	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/model/response"
	"github.com/assimon/luuu/mq"
	"github.com/assimon/luuu/mq/handle"
	"github.com/assimon/luuu/util/constant"
	"github.com/golang-module/carbon/v2"
	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
)

const (
	CnyMinimumPaymentAmount  = 0.01   // cny最低支付金额
	UsdtMinimumPaymentAmount = 0.01   // usdt最低支付金额
	UsdtAmountPerIncrement   = 0.0001 // usdt每次递增金额
	IncrementalMaximumNumber = 100    // 最大递增次数
)

var gCreateTransactionLock sync.Mutex

// CreateTransaction 创建订单
func CreateTransaction(req *request.CreateTransactionRequest) (*response.CreateTransactionResponse, error) {
	gCreateTransactionLock.Lock()
	defer gCreateTransactionLock.Unlock()
	// 按照汇率转化USDT
	decimalPayAmount := req.Amount.Round(4)
	decimalRate := decimal.NewFromFloat(config.GetUsdtRate())

	if req.ForceUsdtRate.Valid {
		decimalRate = req.ForceUsdtRate.Decimal
	}
	decimalUsdt := decimalPayAmount.Div(decimalRate)
	// cny 是否可以满足最低支付金额
	if decimalPayAmount.Cmp(decimal.NewFromFloat(CnyMinimumPaymentAmount)) == -1 {
		return nil, constant.PayAmountErr
	}
	// Usdt是否可以满足最低支付金额
	if decimalUsdt.Cmp(decimal.NewFromFloat(UsdtMinimumPaymentAmount)) == -1 {
		return nil, constant.PayAmountErr
	}
	// 已经存在了的交易
	exist, err := data.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		return nil, err
	}
	if exist.ID > 0 {
		return nil, constant.OrderAlreadyExists
	}
	// 有无可用钱包
	walletAddress, err := data.GetAvailableWalletAddress()
	if err != nil {
		return nil, err
	}
	if len(walletAddress) <= 0 {
		return nil, constant.NotAvailableWalletAddress
	}
	availableToken, availableAmount, err := CalculateAvailableWalletAndAmount(decimalUsdt.Round(4), walletAddress, req.PreferredAddress)
	if err != nil {
		return nil, err
	}
	if availableToken == "" {
		return nil, constant.NotAvailableAmountErr
	}
	tx := dao.Mdb.Begin()
	order := &mdb.Orders{
		TradeId:      GenerateCode(),
		OrderId:      req.OrderId,
		Amount:       req.Amount,
		ActualAmount: availableAmount,
		Token:        availableToken,
		Status:       mdb.StatusWaitPay,
		NotifyUrl:    req.NotifyUrl,
		RedirectUrl:  req.RedirectUrl,
	}
	err = data.CreateOrderWithTransaction(tx, order)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// 锁定支付池
	err = data.LockTransaction(availableToken, order.TradeId, availableAmount, config.GetOrderExpirationTimeDuration())
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	// 超时过期消息队列
	orderExpirationQueue, _ := handle.NewOrderExpirationQueue(order.TradeId)
	mq.MClient.Enqueue(orderExpirationQueue, asynq.ProcessIn(config.GetOrderExpirationTimeDuration()))
	ExpirationTime := carbon.Now().AddMinutes(config.GetOrderExpirationTime()).Timestamp()
	resp := &response.CreateTransactionResponse{
		TradeId:        order.TradeId,
		OrderId:        order.OrderId,
		Amount:         order.Amount,
		ActualAmount:   order.ActualAmount,
		Token:          order.Token,
		ExpirationTime: ExpirationTime,
		PaymentUrl:     fmt.Sprintf("%s/pay/checkout-counter/%s", config.GetAppUri(), order.TradeId),
	}
	return resp, nil
}

// OrderProcessing 成功处理订单
func OrderProcessing(req *request.OrderProcessingRequest) error {
	tx := dao.Mdb.Begin()
	exist, err := data.GetOrderByBlockIdWithTransaction(tx, req.BlockTransactionId)
	if err != nil {
		return err
	}
	if exist.ID > 0 {
		tx.Rollback()
		return constant.OrderBlockAlreadyProcess
	}
	// 标记订单成功
	err = data.OrderSuccessWithTransaction(tx, req)
	if err != nil {
		tx.Rollback()
		return err
	}
	// 解锁交易
	err = data.UnLockTransaction(req.Token, req.Amount)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

// CalculateAvailableWalletAndAmount 计算可用钱包地址和金额
func CalculateAvailableWalletAndAmount(amount decimal.Decimal,
	walletAddress []mdb.WalletAddress, preferredAddress string) (string, decimal.Decimal, error) {
	availableToken := ""
	var fraction = decimal.NewFromInt32(int32(rand.Intn(95) + 5))
	availableAmount := amount.Add(fraction.Div(decimal.NewFromInt32(10000.0)))
	var index = rand.Intn(len(walletAddress))
	if preferredAddress != "" {
		index = From(walletAddress).IndexOf(func(i interface{}) bool {
			return i.(mdb.WalletAddress).Token == preferredAddress
		})
	}

	calculateAvailableWalletFunc := func(amount decimal.Decimal, walletIndex int) (string, error) {
		var address = walletAddress[walletIndex]
		token := address.Token
		result, err := data.GetTradeIdByWalletAddressAndAmount(token, availableAmount)
		if err != nil {
			return "", err
		}
		availableWallet := ""
		if result == "" {
			availableWallet = token
		}

		return availableWallet, nil
	}
	for i := 0; i < IncrementalMaximumNumber; i++ {
		token, err := calculateAvailableWalletFunc(availableAmount, index)
		if err != nil {
			return "", decimal.Zero, err
		}
		// 拿不到可用钱包就累加金额
		if token == "" {
			decimalOldAmount := availableAmount
			decimalIncr := decimal.NewFromFloat(UsdtAmountPerIncrement)
			availableAmount = decimalOldAmount.Add(decimalIncr)
			continue
		}
		availableToken = token
		break
	}
	return availableToken, availableAmount, nil
}

// GenerateCode 订单号生成
func GenerateCode() string {
	date := time.Now().Format("20060102")
	r := rand.Intn(1000)
	code := fmt.Sprintf("%s%d%03d", date, time.Now().UnixNano()/1e6, r)
	return code
}

// GetOrderInfoByTradeId 通过交易号获取订单
func GetOrderInfoByTradeId(tradeId string) (*mdb.Orders, error) {
	order, err := data.GetOrderInfoByTradeId(tradeId)
	if err != nil {
		return nil, err
	}
	if order.ID <= 0 {
		return nil, constant.OrderNotExists
	}
	return order, nil
}

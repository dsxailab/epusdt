package request

import (
	"github.com/gookit/validate"
	"github.com/shopspring/decimal"
)

// CreateTransactionRequest 创建交易请求
type CreateTransactionRequest struct {
	OrderId          string              `json:"order_id" validate:"required|maxLen:128"`
	Amount           decimal.Decimal     `json:"amount" validate:"required|isDecimal"`
	NotifyUrl        string              `json:"notify_url" validate:"required"`
	Signature        string              `json:"signature"  validate:"required"`
	RedirectUrl      string              `json:"redirect_url"`
	ForceUsdtRate    decimal.NullDecimal `json:"force_usdt_rate"`
	PreferredAddress string              `json:"preferred_address"`
}

func (r CreateTransactionRequest) Translates() map[string]string {
	return validate.MS{
		"OrderId":   "订单号",
		"Amount":    "支付金额",
		"NotifyUrl": "异步回调网址",
		"Signature": "签名",
	}
}

// OrderProcessingRequest 订单处理
type OrderProcessingRequest struct {
	Token              string
	Amount             decimal.Decimal
	TradeId            string
	BlockTransactionId string
}

package comm

import (
	"errors"

	"github.com/assimon/luuu/model/request"
	"github.com/assimon/luuu/model/service"
	"github.com/assimon/luuu/util/constant"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// CreateTransaction 创建交易
func (c *BaseCommController) CreateTransaction(ctx echo.Context) (err error) {
	req := new(request.CreateTransactionRequest)
	if err = ctx.Bind(req); err != nil {
		return c.FailJson(ctx, constant.ParamsMarshalErr)
	}
	if err = c.ValidateStruct(ctx, req); err != nil {
		return c.FailJson(ctx, err)
	}
	if req.Amount.LessThan(decimal.NewFromFloat(0.0001)) {
		return c.FailJson(ctx, errors.New("支付金额不足"))
	}
	resp, err := service.CreateTransaction(req)
	if err != nil {
		return c.FailJson(ctx, err)
	}
	return c.SucJson(ctx, resp)
}

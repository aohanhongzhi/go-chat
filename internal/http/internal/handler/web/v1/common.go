package v1

import (
	"go-chat/api/pb/web/v1"
	"go-chat/config"
	"go-chat/internal/entity"
	"go-chat/internal/pkg/ichat"
	"go-chat/internal/service"
)

type Common struct {
	config      *config.Config
	smsService  *service.SmsService
	userService *service.UserService
}

func NewCommon(config *config.Config, smsService *service.SmsService, userService *service.UserService) *Common {
	return &Common{config: config, smsService: smsService, userService: userService}
}

// SmsCode 发送短信验证码
func (c *Common) SmsCode(ctx *ichat.Context) error {

	params := &web.CommonSendSmsRequest{}
	if err := ctx.Context.ShouldBind(params); err != nil {
		return ctx.InvalidParams(err)
	}

	switch params.Channel {
	// 需要判断账号是否存在
	case entity.SmsLoginChannel, entity.SmsForgetAccountChannel:
		if !c.userService.Dao().IsMobileExist(params.Mobile) {
			return ctx.ErrorBusiness("账号不存在或密码错误！")
		}

	// 需要判断账号是否存在
	case entity.SmsRegisterChannel, entity.SmsChangeAccountChannel:
		if c.userService.Dao().IsMobileExist(params.Mobile) {
			return ctx.ErrorBusiness("手机号已被他人使用！")
		}

	default:
		return ctx.ErrorBusiness("发送异常！")
	}

	// 发送短信验证码
	code, err := c.smsService.Send(ctx.Ctx(), params.Channel, params.Mobile)
	if err != nil {
		return ctx.ErrorBusiness(err.Error())
	}

	if params.Channel == entity.SmsRegisterChannel {
		return ctx.Success(entity.MapStrAny{
			"is_debug": true,
			"sms_code": code,
		})
	}

	return ctx.Success(&web.CommonSendSmsResponse{})
}

// EmailCode 发送邮件验证码
func (c *Common) EmailCode(ctx *ichat.Context) error {

	params := &web.CommonSendEmailRequest{}
	if err := ctx.Context.ShouldBind(params); err != nil {
		return ctx.InvalidParams(err)
	}

	return ctx.Success(nil)
}

// Setting 公共设置
func (c *Common) Setting(ctx *ichat.Context) error {
	return nil
}

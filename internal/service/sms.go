package service

import (
	"context"
	"fmt"
	"time"

	"go-chat/internal/pkg/strutil"
	"go-chat/internal/repository/cache"
)

type SmsService struct {
	smsCodeCache *cache.SmsCodeCache
}

func NewSmsService(codeCache *cache.SmsCodeCache) *SmsService {
	return &SmsService{smsCodeCache: codeCache}
}

// Check 验证短信验证码是否正确
func (s *SmsService) Check(ctx context.Context, channel string, mobile string, code string) bool {
	value, err := s.smsCodeCache.Get(ctx, channel, mobile)

	if err != nil || value == "" {
		return false
	}

	if value == code {
		return true
	}

	// 验证失败超过5次，则直接删除缓存
	failNum := s.smsCodeCache.IncrVerifyFail(ctx, channel, mobile, 15*time.Minute)
	if failNum >= 5 {
		_ = s.smsCodeCache.Del(ctx, channel, mobile)
	}

	return false
}

// Delete 删除短信验证码记录
func (s *SmsService) Delete(ctx context.Context, channel string, mobile string) {
	_ = s.smsCodeCache.Del(ctx, channel, mobile)
}

// Send 发送短信
func (s *SmsService) Send(ctx context.Context, channel string, mobile string) (string, error) {

	code := strutil.GenValidateCode(6)

	// 添加发送记录
	if err := s.smsCodeCache.Set(ctx, channel, mobile, code, 15*time.Minute); err != nil {
		return "", err
	}

	// TODO ... 请求第三方短信接口
	fmt.Println("正在发送短信验证码：", code)

	return code, nil
}

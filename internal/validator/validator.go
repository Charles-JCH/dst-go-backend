package validator

import (
	"errors"
	"github.com/gin-gonic/gin/binding"
	playground "github.com/go-playground/validator/v10"
	"regexp"
)

var (
	phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
	// linuxUserPattern Linux 用户名：小写字母或下划线开头，最长 32 位
	linuxUserPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	// singleLinePattern 不含换行等控制字符，内容会写入游戏配置文件，换行会破坏文件格式
	singleLinePattern = regexp.MustCompile(`^[^\p{Cc}]*$`)
)

// Register 注册自定义的参数校验规则，需要在创建路由前调用
func Register() error {
	engine, ok := binding.Validator.Engine().(*playground.Validate)
	if !ok {
		return errors.New("参数校验引擎类型不匹配")
	}
	if err := engine.RegisterValidation("cnphone", func(fl playground.FieldLevel) bool {
		return phonePattern.MatchString(fl.Field().String())
	}); err != nil {
		return err
	}
	if err := engine.RegisterValidation("linuxuser", func(fl playground.FieldLevel) bool {
		return IsLinuxUsername(fl.Field().String())
	}); err != nil {
		return err
	}
	return engine.RegisterValidation("singleline", func(fl playground.FieldLevel) bool {
		return singleLinePattern.MatchString(fl.Field().String())
	})
}

// IsLinuxUsername 是否为合法的 Linux 用户名
func IsLinuxUsername(s string) bool {
	return linuxUserPattern.MatchString(s)
}

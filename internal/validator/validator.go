package validator

import (
	"errors"
	"github.com/gin-gonic/gin/binding"
	playground "github.com/go-playground/validator/v10"
	"regexp"
)

var phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

// Register 注册自定义的参数校验规则，需要在创建路由前调用
func Register() error {
	engine, ok := binding.Validator.Engine().(*playground.Validate)
	if !ok {
		return errors.New("参数校验引擎类型不匹配")
	}
	return engine.RegisterValidation("cnphone", func(fl playground.FieldLevel) bool {
		return phonePattern.MatchString(fl.Field().String())
	})
}

package main

import (
	"fmt"

	"github.com/assimon/luuu/bootstrap"
	"github.com/assimon/luuu/config"
	"github.com/gookit/color"
	"github.com/gookit/validate"
	"github.com/shopspring/decimal"
)

func main() {
	validate.AddValidator("isDecimal", func(val interface{}) bool {
		var _, err = decimal.NewFromString(fmt.Sprintf("%v", val))
		return err == nil
	})

	defer func() {
		if err := recover(); err != nil {
			color.Error.Println("[Start Server Err!!!] ", err)
		}
	}()
	color.Green.Printf("%s\n", "  _____                     _ _   \n | ____|_ __  _   _ ___  __| | |_ \n |  _| | '_ \\| | | / __|/ _` | __|\n | |___| |_) | |_| \\__ \\ (_| | |_ \n |_____| .__/ \\__,_|___/\\__,_|\\__|\n       |_|                        ")
	color.Infof("Epusdt version(%s) Powered by %s %s \n", config.GetAppVersion(), "assimon", "https://github.com/assimon/epusdt")
	bootstrap.Start()
}

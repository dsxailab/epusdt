package http_client

import (
	"github.com/assimon/luuu/config"
	"github.com/go-resty/resty/v2"
)

// GetHttpClient 获取请求客户端
func GetHttpClient(proxys ...string) *resty.Client {
	client := resty.New()
	// 如果有代理
	if len(proxys) > 0 {
		proxy := proxys[0]
		client.SetProxy(proxy)
	}
	client.SetTimeout(config.GetTrcQryTimeout())
	return client
}

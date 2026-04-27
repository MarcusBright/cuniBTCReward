package setup

import (
	"cuniBTCReward/api/internal/config"
	"cuniBTCReward/api/internal/handler"
	"cuniBTCReward/api/internal/svc"
	setupconfig "cuniBTCReward/api/setup/config"
	"unsafe"

	"github.com/zeromicro/go-zero/rest"
)

func Setup(server *rest.Server, c setupconfig.Config) {
	config := (*config.Config)(unsafe.Pointer(&c))
	ctx := svc.NewServiceContext(*config)
	handler.RegisterHandlers(server, ctx)
}

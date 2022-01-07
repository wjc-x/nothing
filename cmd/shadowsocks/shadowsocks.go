package shadowsocks

import "C"
import (
	"context"

	"github.com/wjc-x/nothing/api"
	"github.com/wjc-x/nothing/core"
	"github.com/wjc-x/nothing/stat"
)

var (
	ctx    context.Context
	cancel context.CancelFunc
)

func StartGoShadowsocks(ClientAddr string, ServerAddr string, Cipher string, Password string, Plugin string, PluginOptions string, EnableAPI bool, APIAddress string) {

	var err error
	addr := ServerAddr

	ctx, cancel = context.WithCancel(context.Background())

	var key []byte

	ciph, err := core.PickCipher(Cipher, key, Password)
	if err != nil {
	}

	if Plugin != "" {
		addr, err = startPlugin(Plugin, PluginOptions, addr, false)
		if err != nil {
		}
	}

	meter := &stat.MemoryTrafficMeter{}

	if EnableAPI {
		go api.RunClientAPIService(ctx, APIAddress, meter)
	}

	go socksLocal(ClientAddr, addr, meter, ciph.StreamConn, ctx)
	go udpSocksLocal(ClientAddr, addr, ciph.PacketConn, ctx)

}

func StopGoShadowsocks() {
	killPlugin()

	cancel()

	closeTcpLocal()
	closeUdpLocal()
}

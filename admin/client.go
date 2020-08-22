package admin

import (
	"encoding/json"
	"log"

	"github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nconnect/config"
	"github.com/nknorg/nconnect/util"
	tunnel "github.com/nknorg/nkn-tunnel"
)

var (
	clientAddr string
)

func StartClient(account *nkn.Account, identifier string, clientConfig *nkn.ClientConfig, tun *tunnel.Tunnel, conf *config.Config) error {
	m, err := nkn.NewMultiClient(account, identifier, 4, false, clientConfig)
	if err != nil {
		return err
	}

	<-m.OnConnect.C

	clientAddr = m.Address()

	for {
		msg := <-m.OnMessage.C

		req := &rpcReq{}
		err := json.Unmarshal(msg.Data, req)
		if err != nil {
			log.Println("Unmarshal client request error:", err)
			continue
		}

		if !util.MatchRegex(conf.GetAdminAddrs(), msg.Src) && !tokenStore.IsValid(req.Token) {
			log.Println("Ignore authorized message from", msg.Src)
			continue
		}

		resp := handleRequest(req, conf, tun)

		b, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			continue
		}

		err = msg.Reply(string(b))
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

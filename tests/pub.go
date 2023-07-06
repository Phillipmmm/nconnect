package tests

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/cakturk/go-netstat/netstat"
	"github.com/nknorg/nconnect"
	"github.com/nknorg/nconnect/config"
	nkn "github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/nknorg/tuna"
	"github.com/nknorg/tuna/pb"
	"github.com/nknorg/tuna/types"
	"github.com/nknorg/tuna/util"
)

var ch chan string = make(chan string, 4)

func startNconnect(configFile string, tuna, udp, tun bool, n *types.Node) error {
	b, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("read config file %v err: %v", configFile, err)
		return err
	}
	var opts = &config.Opts{}
	err = json.Unmarshal(b, opts)
	if err != nil {
		log.Fatalf("parse config %v err: %v", configFile, err)
		return err
	}

	opts.Config.Tuna = tuna
	opts.Config.UDP = udp
	opts.Config.Tun = tun
	if tun {
		opts.Config.VPN = true
	}

	if opts.Client {
		port, err := getFreePort(port)
		if err != nil {
			return err
		}
		opts.LocalSocksAddr = fmt.Sprintf("127.0.0.1:%v", port)
	}
	fmt.Printf("opts.RemoteAdminAddr: %+v\n", opts.RemoteAdminAddr)

	nc, _ := nconnect.NewNconnect(opts)
	if opts.Server {
		nc.SetTunaNode(n)
		err = nc.StartServer()
	} else {
		err = nc.StartClient()
	}
	return err
}

func getTunaNode() (*types.Node, error) {
	tunaSeed, _ := hex.DecodeString(seedHex)
	acc, err := nkn.NewAccount(tunaSeed)
	if err != nil {
		return nil, err
	}

	go runReverseEntry(tunaSeed)

	md := &pb.ServiceMetadata{
		Ip:              "127.0.0.1",
		TcpPort:         30020,
		UdpPort:         30021,
		ServiceId:       0,
		Price:           "0.0",
		BeneficiaryAddr: "",
	}
	n := &types.Node{
		Delay:       0,
		Bandwidth:   0,
		Metadata:    md,
		Address:     hex.EncodeToString(acc.PublicKey),
		MetadataRaw: "CgkxMjcuMC4wLjEQxOoBGMXqAToFMC4wMDE=",
	}

	return n, nil
}

func runReverseEntry(seed []byte) error {
	entryAccount, err := vault.NewAccountWithSeed(seed)
	if err != nil {
		return err
	}
	seedRPCServerAddr := nkn.NewStringArray(nkn.DefaultSeedRPCServerAddr...)

	walletConfig := &nkn.WalletConfig{
		SeedRPCServerAddr: seedRPCServerAddr,
	}
	entryWallet, err := nkn.NewWallet(&nkn.Account{Account: entryAccount}, walletConfig)
	if err != nil {
		return err
	}

	entryConfig := new(tuna.EntryConfiguration)
	err = util.ReadJSON("config.reverse.entry.json", entryConfig)
	if err != nil {
		return err
	}

	err = tuna.StartReverse(entryConfig, entryWallet)
	if err != nil {
		return err
	}

	ch <- tunaNodeStarted

	select {}
}

type Person struct {
	Name string
	Age  int
}

func getFreePort(port int) (int, error) {
	for i := 0; i < 100; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%v", port))
		if err != nil {
			return 0, err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			port++
			continue
		}

		defer l.Close()

		return l.Addr().(*net.TCPAddr).Port, nil
	}
	return 0, fmt.Errorf("can't find free port")
}

func waitSSAndTunaReady() error {
	ssIsReady := false
	for i := 0; i < 100; i++ {
		tabs, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
			return s.State == netstat.Listen && s.LocalAddr.Port == uint16(port)
		})
		if err != nil {
			fmt.Printf("waitSSAndTunaReady err: %v\n", err)
		}
		if len(tabs) >= 1 {
			ssIsReady = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !ssIsReady {
		return fmt.Errorf("ss is not ready after 200 seconds, give up")
	}

	for i := 0; i < 100; i++ {
		tabs, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
			return s.State == netstat.Established && s.RemoteAddr.Port == 30020
		})
		if err != nil {
			fmt.Printf("waitSSAndTunaReady err: %v\n", err)
		}
		time.Sleep(2 * time.Second)
		if len(tabs) >= 1 {
			return nil
		}
	}

	return fmt.Errorf("tuna is not connected after 200 seconds, give up")
}

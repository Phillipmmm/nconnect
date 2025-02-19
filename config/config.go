package config

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/nknorg/nconnect/util"
	"github.com/nknorg/nkn/v2/common"
)

const (
	RandomIdentifierChars  = "abcdefghijklmnopqrstuvwxyz0123456789"
	RandomIdentifierLength = 6
	DefaultTunNameLinux    = "nConnect-tun0"
	DefaultTunNameNonLinux = "nConnect-tap0"
	FallbackTunaMaxPrice   = "0.01"
	DefaultUDPTimeout      = time.Hour * 720
)

var (
	Version string
)

func init() {
	rand.NewSource(time.Now().UnixNano())
}

type Opts struct {
	Client bool `short:"c" long:"client" description:"Client mode"`
	Server bool `short:"s" long:"server" description:"Server mode"`

	Config
	ConfigFile string `short:"f" long:"config-file" default:"config.json" description:"Config file path"`

	Address       bool `long:"address" description:"Print client address (client mode) or admin address (server mode)"`
	WalletAddress bool `long:"wallet-address" description:"Print wallet address (server only)"`
	Version       bool `long:"version" description:"Print version"`
}

type Config struct {
	path string

	// Account config
	Identifier string `json:"identifier" long:"identifier" description:"NKN client identifier. A random one will be generated and saved to config.json if not provided."`
	Seed       string `json:"seed" long:"seed" description:"NKN client secret seed. A random one will be generated and saved to config.json if not provided."`

	// NKN Client config
	SeedRPCServerAddr []string `json:"seedRPCServerAddr,omitempty" long:"rpc" description:"Seed RPC server address"`
	ConnectRetries    int32    `json:"connectRetries,omitempty" long:"connect-retries" description:"client connect retries, a negative value means unlimited retries."`

	// Cipher config
	Cipher   string `json:"cipher,omitempty" long:"cipher" description:"Socks proxy cipher. Dummy (no cipher) will not reduce security because NKN tunnel already has end to end encryption." choice:"dummy" choice:"chacha20-ietf-poly1305" choice:"aes-128-gcm" choice:"aes-256-gcm" default:"chacha20-ietf-poly1305"`
	Password string `json:"password,omitempty" long:"password" description:"Socks proxy password"`

	// Session config
	DialTimeout       int32 `json:"dialTimeout,omitempty" long:"dial-timeout" description:"dial timeout in milliseconds"`
	SessionWindowSize int32 `json:"sessionWindowSize,omitempty" long:"session-window-size" description:"tuna session window size (byte)."`

	// Log config
	LogFileName        string `json:"log,omitempty" long:"log" description:"Log file path. Will write log to stdout if not provided."`
	LogMaxSize         int    `json:"logMaxSize,omitempty" long:"log-max-size" description:"Maximum size in megabytes of the log file before it gets rotated." default:"1"`
	LogMaxBackups      int    `json:"logMaxBackups,omitempty" long:"log-max-backups" description:"Maximum number of old log files to retain." default:"3"`
	LogAPIResponseSize int    `json:"logAPIResponseSize,omitempty" long:"log-api-response-size" description:"(server only) Maximum size in bytes of get log api response. If log size is greater than this value, only the lastest part of the log will be returned."`

	// Remote address
	RemoteAdminAddr  []string `json:"remoteAdminAddr,omitempty" short:"a" long:"remote-admin-addr" description:"(client only) Remote server admin address"`
	RemoteTunnelAddr []string `json:"remoteTunnelAddr,omitempty" short:"r" long:"remote-tunnel-addr" description:"(client only) Remote server tunnel address, not needed if remote server admin address is given"`

	// Socks proxy config
	LocalSocksAddr string `json:"localSocksAddr,omitempty" short:"l" long:"local-socks-addr" description:"(client only) Local socks proxy listen address" default:"127.0.0.1:1080"`

	// TUN/TAP device config
	Tun        bool     `json:"tun,omitempty" long:"tun" description:"(client only) Enable TUN device, might require root privilege"`
	TunAddr    string   `json:"tunAddr,omitempty" long:"tun-addr" description:"(client only) TUN device IP address" default:"10.0.86.2"`
	TunGateway string   `json:"tunGateway,omitempty" long:"tun-gateway" description:"(client only) TUN device gateway" default:"10.0.86.1"`
	TunMask    string   `json:"tunMask,omitempty" long:"tun-mask" description:"(client only) TUN device network mask, should be a prefixlen (a number) for IPv6 address" default:"255.255.255.0"`
	TunDNS     []string `json:"tunDNS,omitempty" long:"tun-dns" description:"(client only) DNS resolvers for the TUN device (Windows only)" default:"1.1.1.1" default:"8.8.8.8"`
	TunName    string   `json:"tunName,omitempty" long:"tun-name" description:"(client only) TUN device name, will be ignored on MacOS. Default is nConnect-tun0 on Linux and nConnect-tap0 on Windows."`

	// VPN mode config
	VPN      bool     `json:"vpn,omitempty" long:"vpn" description:"(client only) Enable VPN mode, might require root privilege. TUN device will be enabled when VPN mode is enabled."`
	VPNRoute []string `json:"vpnRoute,omitempty" long:"vpn-route" description:"(client only) VPN routing table destinations, each item should be a valid CIDR. If not given, remote server's local IP addresses will be used."`

	// Tuna config
	Tuna                        bool     `json:"tuna,omitempty" short:"t" long:"tuna" description:"Enable tuna sessions"`
	TunaMinBalance              string   `json:"tunaMinBalance,omitempty" long:"tuna-min-balance" description:"(server only) Minimal balance to enable tuna sessions" default:"0.01"`
	TunaMaxPrice                string   `json:"tunaMaxPrice,omitempty" long:"tuna-max-price" description:"(server only) Tuna max price in unit of NKN/MB. Can also be a url where the price will be get dynamically at launch." default:"0.01"`
	TunaMinFee                  string   `json:"tunaMinFee,omitempty" long:"tuna-min-fee" description:"(server only) Tuna nanopay minimal txn fee" default:"0.00001"`
	TunaFeeRatio                float64  `json:"tunaFeeRatio,omitempty" long:"tuna-fee-ratio" description:"(server only) Tuna nanopay txn fee ratio" default:"0.1"`
	TunaCountry                 []string `json:"tunaCountry,omitempty" long:"tuna-country" description:"(server only) Tuna service node allowed country code, e.g. US. All countries will be allowed if not provided"`
	TunaServiceName             string   `json:"tunaServiceName,omitempty" long:"tuna-service-name" description:"(server only) Tuna reverse service name"`
	TunaAllowNknAddr            []string `json:"tunaAllowNknAddr,omitempty" long:"tuna-allow-nkn-addr" description:"(server only) Tuna service node allowed NKN address. All NKN address will be allowed if not provided"`
	TunaDisallowNknAddr         []string `json:"tunaDisallowNknAddr,omitempty" long:"tuna-disallow-nkn-addr" description:"(server only) Tuna service node disallowed NKN address. All NKN address will be allowed if not provided"`
	TunaAllowIp                 []string `json:"tunaAllowIp,omitempty" long:"tuna-allow-ip" description:"(server only) Tuna service node allowed IP. All IP will be allowed if not provided"`
	TunaDisallowIp              []string `json:"tunaDisallowIp,omitempty" long:"tuna-disallow-ip" description:"(server only) Tuna service node disallowed IP. All IP will be allowed if not provided"`
	TunaDisableDownloadGeoDB    bool     `json:"tunaDisableDownloadGeoDB,omitempty" long:"tuna-disable-download-geo-db" description:"(server only) Disable Tuna download geo db to disk"`
	TunaGeoDBPath               string   `json:"tunaGeoDBPath,omitempty" long:"tuna-geo-db-path" description:"(server only) Path to store Tuna geo db" default:"."`
	TunaDisableMeasureBandwidth bool     `json:"tunaDisableMeasureBandwidth,omitempty" long:"tuna-disable-measure-bandwidth" description:"(server only) Disable Tuna measure bandwidth when selecting service nodes"`
	TunaMeasureStoragePath      string   `json:"tunaMeasureStoragePath,omitempty" long:"tuna-measure-storage-path" description:"(server only) Path to store Tuna measurement results" default:"."`
	TunaMeasureBandwidthBytes   int32    `json:"tunaMeasureBandwidthBytes,omitempty" long:"tuna-measure-bandwidth-bytes" description:"(server only) Tuna measure bandwidth bytes to transmit when selecting service nodes" default:"1"`

	// UDP config
	UDP         bool  `json:"udp,omitempty" long:"udp" description:"Support udp proxy"`
	UDPIdleTime int32 `json:"udpIdleTime,omitempty" long:"udp-idle-time" description:"UDP connections will be purged after idle time (in seconds). 0 is for no purge" default:"0"`

	// Admin config
	AdminIdentifier     string `json:"adminIdentifier,omitempty" long:"admin-identifier" description:"(server only) Admin NKN client identifier prefix" default:"nConnect"`
	AdminHTTPAddr       string `json:"adminHttpAddr,omitempty" long:"admin-http" description:"(server only) Admin web GUI listen address (e.g. 127.0.0.1:8000)"`
	DisableAdminHTTPAPI bool   `json:"disableAdminHttpApi,omitempty" long:"disable-admin-http-api" description:"(server only) Disable admin http api so admin web GUI only show static assets"`
	WebRootPath         string `json:"webRootPath,omitempty" long:"web-root-path" description:"(server only) Web root path" default:"web/dist"`

	Tags    []string `json:"tags,omitempty" long:"tags" description:"(server only) Tags that will be included in get info api"`
	Verbose bool     `json:"verbose,omitempty" short:"v" long:"verbose" description:"Verbose mode, show logs on dialing/accepting connections"`

	lock        sync.RWMutex
	AcceptAddrs []string `json:"acceptAddrs"`
	AdminAddrs  []string `json:"adminAddrs"`
}

func NewConfig() *Config {
	return &Config{
		AcceptAddrs: make([]string, 0),
		AdminAddrs:  make([]string, 0),
	}
}

func LoadOrNewConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c := NewConfig()
			c.path = path
			err := c.save()
			if err != nil {
				return nil, err
			}
			return c, nil
		}
		return nil, err
	}

	c := &Config{
		path: path,
	}

	err = json.Unmarshal(b, c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Config) SetPlatformSpecificDefaultValues() error {
	if len(c.TunName) == 0 {
		switch runtime.GOOS {
		case "linux":
			c.TunName = DefaultTunNameLinux
		default:
			c.TunName = DefaultTunNameNonLinux
		}
	}
	return nil
}

func (c *Config) VerifyClient() error {
	if len(c.RemoteAdminAddr) == 0 && len(c.RemoteTunnelAddr) == 0 {
		return errors.New("remoteAdminAddr and remoteTunnelAddr are both empty")
	}
	return nil
}

func (c *Config) VerifyServer() error {
	_, err := common.StringToFixed64(c.TunaMinBalance)
	if err != nil {
		return fmt.Errorf("parse TunaMinBalance error: %v", err)
	}
	_, err = common.StringToFixed64(c.TunaMaxPrice)
	if err != nil {
		return fmt.Errorf("parse TunaMaxPrice error: %v", err)
	}
	_, err = common.StringToFixed64(c.TunaMinFee)
	if err != nil {
		return fmt.Errorf("parse TunaMinFee error: %v", err)
	}
	return nil
}

func (c *Config) GetAcceptAddrs() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.AcceptAddrs
}

func (c *Config) SetAcceptAddrs(acceptAddrs []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.AcceptAddrs = acceptAddrs
	return c.save()
}

func (c *Config) AddAcceptAddrs(acceptAddrs []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.AcceptAddrs = util.MergeStrings(c.AcceptAddrs, acceptAddrs)
	return c.save()
}

func (c *Config) RemoveAcceptAddrs(acceptAddrs []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.AcceptAddrs = util.RemoveStrings(c.AcceptAddrs, acceptAddrs)
	return c.save()
}

func (c *Config) GetAdminAddrs() []string {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.AdminAddrs
}

func (c *Config) SetAdminAddrs(adminAddrs []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.AdminAddrs = adminAddrs
	return c.save()
}

func (c *Config) AddAdminAddrs(adminAddrs []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.AdminAddrs = util.MergeStrings(c.AdminAddrs, adminAddrs)
	return c.save()
}

func (c *Config) RemoveAdminAddrs(adminAddrs []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.AdminAddrs = util.RemoveStrings(c.AdminAddrs, adminAddrs)
	return c.save()
}

func (c *Config) SetAdminHTTPAPI(disable bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.DisableAdminHTTPAPI = disable
	return c.save()
}

func (c *Config) SetSeed(s string) error {
	seed, err := hex.DecodeString(s)
	if err != nil {
		return errors.New("invalid seed string, should be a hex string")
	}

	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("invalid seed string length %d, should be %d", len(s), 2*ed25519.SeedSize)
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	c.Seed = s
	return c.save()
}

func (c *Config) SetTunaConfig(serviceName string, country []string, allowNknAddr []string, disallowNknAddr []string, allowIp []string, disallowIp []string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.TunaServiceName = serviceName
	c.TunaCountry = country
	c.TunaAllowNknAddr = allowNknAddr
	c.TunaDisallowNknAddr = disallowNknAddr
	c.TunaAllowIp = allowIp
	c.TunaDisallowIp = disallowIp
	return c.save()
}

func (c *Config) Save() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.save()
}

func (c *Config) save() error {
	if len(c.path) == 0 {
		return nil
	}

	b, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}

	err = os.WriteFile(c.path, b, 0666)
	if err != nil {
		return err
	}

	return nil
}

func RandomIdentifier() string {
	b := make([]byte, RandomIdentifierLength)
	for i := range b {
		b[i] = RandomIdentifierChars[rand.Intn(len(RandomIdentifierChars))]
	}
	return string(b)
}

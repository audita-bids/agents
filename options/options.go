package options

import (
	"flag"
)

type Cfg struct {
	Debug    bool
	GRPCAddr string
	HttpAddr string
	PromAddr string
}

func NewCfg() *Cfg {
	return &Cfg{
		Debug:    true,
		GRPCAddr: ":9090",
		HttpAddr: ":8080",
		PromAddr: ":19090",
	}
}

func HandleCfg(cfg *Cfg) *Cfg {
	flag.BoolVar(&cfg.Debug, "debug", cfg.Debug, "Enable Debug into the server")
	flag.StringVar(&cfg.GRPCAddr, "grpc-addr", cfg.GRPCAddr, "GRPC address to listen on")
	flag.StringVar(&cfg.HttpAddr, "http-addr", cfg.HttpAddr, "HTTP address to listen on")
	flag.StringVar(&cfg.PromAddr, "prom-addr", cfg.PromAddr, "Prometheus address to liste on")

	flag.Parse()

	return cfg
}

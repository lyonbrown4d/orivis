package main

import (
	"fmt"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DB struct {
		ResultRetention string
	} `mapstructure:"db"`
}

func main() {
	m := map[string]any{"db.result_retention": "24h", "db.ResultRetention": "xx"}
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(m, "."), nil); err != nil {
		panic(err)
	}
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		panic(err)
	}
	if _, err := fmt.Printf("%#v\n", cfg); err != nil {
		panic(err)
	}
}

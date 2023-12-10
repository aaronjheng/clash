package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	_ "time/tzdata"

	_ "github.com/kr/pretty"
	"github.com/spf13/cobra"
	"github.com/square/exit"
	"go.uber.org/automaxprocs/maxprocs"

	"github.com/Dreamacro/clash/internal/config"
	C "github.com/Dreamacro/clash/internal/constant"
	"github.com/Dreamacro/clash/internal/hub"
	"github.com/Dreamacro/clash/internal/hub/executor"
	"github.com/Dreamacro/clash/internal/log"
	internalversion "github.com/Dreamacro/clash/internal/version"
)

var (
	version            bool
	testConfig         bool
	configFile         string
	externalController string
	secret             string
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "clash",
		RunE: func(cmd *cobra.Command, args []string) error {
			if version {
				fmt.Printf("Clash %s %s %s with %s\n", internalversion.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
				return nil
			}

			if configFile != "" {
				if !filepath.IsAbs(configFile) {
					currentDir, _ := os.Getwd()
					configFile = filepath.Join(currentDir, configFile)
				}
				C.SetConfig(configFile)
			} else {
				configFile := filepath.Join(C.Path.HomeDir(), C.Path.Config())
				C.SetConfig(configFile)
			}

			if err := config.Init(C.Path.HomeDir(), C.Path.CacheDir(), C.Path.StateDir()); err != nil {
				log.Fatalln("Initial configuration directory error: %s", err.Error())
			}

			if testConfig {
				if _, err := executor.Parse(); err != nil {
					log.Errorln(err.Error())
					fmt.Printf("configuration file %s test failed\n", C.Path.Config())
					os.Exit(1)
				}
				fmt.Printf("configuration file %s test is successful\n", C.Path.Config())
				return nil
			}

			var options []hub.Option
			if externalController != "" {
				options = append(options, hub.WithExternalController(externalController))
			}
			if secret != "" {
				options = append(options, hub.WithSecret(secret))
			}

			if err := hub.Parse(options...); err != nil {
				log.Fatalln("Parse config error: %s", err.Error())
			}

			// srv := server.New()
			// if err := srv.Serve(); err != nil {
			// 	return fmt.Errorf("server error: %w", err)
			// }

			termSign := make(chan os.Signal, 1)
			hupSign := make(chan os.Signal, 1)
			signal.Notify(termSign, syscall.SIGINT, syscall.SIGTERM)
			signal.Notify(hupSign, syscall.SIGHUP)
			for {
				select {
				case <-termSign:
					return nil
				case <-hupSign:
					if cfg, err := executor.ParseWithPath(C.Path.Config()); err == nil {
						executor.ApplyConfig(cfg, true)
					} else {
						log.Errorln("Parse config error: %s", err.Error())
					}
				}
			}
		},
	}

	flagSet := cmd.Flags()
	flagSet.StringVarP(&configFile, "config", "f", "", "Configuration file path")

	return cmd
}

func main() {
	maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))

	cmd := rootCmd()
	if err := cmd.Execute(); err != nil {
		log.Errorln("Clash failed: %v", err)

		os.Exit(exit.NotOK)
	}
}

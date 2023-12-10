package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	_ "time/tzdata"

	_ "github.com/kr/pretty"
	"github.com/spf13/cobra"
	"github.com/square/exit"
	"go.uber.org/automaxprocs/maxprocs"

	C "github.com/Dreamacro/clash/internal/constant"
	"github.com/Dreamacro/clash/internal/hub/executor"
	"github.com/Dreamacro/clash/internal/log"
	"github.com/Dreamacro/clash/internal/server"
	internalversion "github.com/Dreamacro/clash/internal/version"
)

var (
	version    bool
	testConfig bool
	configFile string
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "clash",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

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

			if testConfig {
				if _, err := executor.Parse(); err != nil {
					log.Errorln(err.Error())
					fmt.Printf("configuration file %s test failed\n", C.Path.Config())
					os.Exit(exit.NotOK)
				}

				fmt.Printf("configuration file %s test succeeded\n", C.Path.Config())
				return nil
			}

			srv := server.New()

			if err := srv.Bootstrap(C.Path.HomeDir(), C.Path.CacheDir(), C.Path.StateDir()); err != nil {
				return fmt.Errorf("server bootstrap failed: %w", err)
			}

			if err := srv.Serve(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					log.Infoln("Clash Server stopped")
				} else {
					return fmt.Errorf("server error: %w", err)
				}
			}

			return nil
		},
	}

	flagSet := cmd.Flags()
	flagSet.StringVarP(&configFile, "config", "f", "", "Configuration file path")
	flagSet.BoolVarP(&version, "version", "V", false, "Clash version")
	flagSet.BoolVarP(&testConfig, "test-config", "t", false, "Config testing")

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

package cmd

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/caitunai/go-blueprint/api/server"
	"github.com/caitunai/go-blueprint/cache"
	"github.com/caitunai/go-blueprint/queue"
	"github.com/caitunai/go-blueprint/redis"
)

// ErrServeCommand indicates run serve command failed.
var ErrServeCommand = errors.New("run serve command failed")

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Command to start api server",
	Long:  "Start the server, you should set the config file, named: .app.toml",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if err := validateConfigCenterSettings(); err != nil {
			return err
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := redis.Init(cmd.Context()); err != nil {
			return errors.Join(ErrServeCommand, err)
		}
		cache.InitCache()
		err := queue.Init()
		if err != nil {
			log.Error().
				Err(err).
				Str("method", "Run").
				Str("package", "cmd").
				Str("command", "serve").
				Msg("init the queue publisher failed")
			return errors.Join(ErrServeCommand, err)
		}
		s := server.NewServer(viper.GetString("port"), viper.GetString("mode"))
		s.Start(cmd.Context())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

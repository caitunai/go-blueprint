package cmd

import (
	"github.com/caitunai/go-blueprint/api/server"
	"github.com/caitunai/go-blueprint/cache"
	"github.com/caitunai/go-blueprint/queue"
	"github.com/caitunai/go-blueprint/redis"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Command to start api server",
	Long:  "Start the server, you should set the config file, named: .app.toml",
	Run: func(cmd *cobra.Command, args []string) {
		redis.Init()
		cache.InitCache()
		err := queue.Init()
		if err != nil {
			log.Error().
				Err(err).
				Str("method", "Run").
				Str("package", "cmd").
				Str("command", "serve").
				Msg("init the queue publisher failed")
		}
		s := server.NewServer(viper.GetString("port"), viper.GetString("mode"))
		s.Start(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

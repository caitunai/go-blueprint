package cmd

import (
	"os"
	"strings"

	"github.com/caitunai/go-blueprint/db"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "A golang application",
	Long:  "A golang application server, with api and website ui",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"config file (default is $HOME/.app.toml)",
	)
}

func initConfig() {
	if cfgFile != "" {
		// Use a config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		// Search config in home directory with name ".ai-canvas" (without extension).
		viper.AddConfigPath(".")
		viper.AddConfigPath(home)
		viper.SetConfigType("toml")
		viper.SetConfigName(".app")
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		log.Trace().Msgf("Using config file: %s", viper.ConfigFileUsed())
		if viper.GetString("db.database") != "" {
			db.Conn()
		}
	} else {
		log.Trace().Err(err).Send()
	}
	initLogger()
}

func initLogger() {
	if viper.GetString("mode") != "dev" {
		switch viper.GetString("logger.level") {
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		}
	} else {
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr}
		log.Logger = log.Output(consoleWriter)
	}
	zerolog.DefaultContextLogger = &log.Logger
}

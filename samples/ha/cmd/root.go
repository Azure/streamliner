/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package cmd

import (
	"fmt"
	"os"

	config "github.com/Azure/streamliner/samples/ha/pkg/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "github.com/Azure/streamliner/samples/ha",
	Short: "A brief description of your application",
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
	rootCmd.PersistentFlags().AddFlagSet(config.GetGenericFlags())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if config.GetConfigFileName() != "" {
		// Use config file from the flag.
		viper.SetConfigFile(config.GetConfigFileName())
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".config" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".config")
		viper.SetConfigName("config")
	}
	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Info().Msg(fmt.Sprintf("Using config file: %s", viper.ConfigFileUsed()))
	} else {
		log.Error().Msg(err.Error())
	}

	viper.SetEnvPrefix(config.EnvPrefix)

	viper.AutomaticEnv() // read in environment variables that match

	config.BindFlags(rootCmd, viper.GetViper())
}

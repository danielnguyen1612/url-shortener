package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/danielnguyentb/url-shortener/core"
	"github.com/danielnguyentb/url-shortener/libs"
	"github.com/danielnguyentb/url-shortener/middlewares"
	"github.com/danielnguyentb/url-shortener/server"
)

const (
	name    = "url-shortener"
	portKey = "server.port"
)

func main() {
	log := libs.InitLogging()
	rootCmd := &cobra.Command{
		Use:   name,
		Short: "Custom webapp",
	}
	rootCmd.AddCommand(serveCommand(log))
	libs.PreExecuteConfiguration(rootCmd, name, log)
	libs.Execute(rootCmd, log)
}

func serveCommand(zapLogger *zap.Logger) *cobra.Command {
	command := &cobra.Command{
		Use:   "serve",
		Short: "Start server",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := core.NewRouter()

			// Add middleware
			r.Use(middlewares.Timeout(time.Minute))
			r.Use(middlewares.Recoverer)
			r.Use(libs.NewZapLogEntry(zapLogger))
			r.Use(middlewares.AllowContentType("application/json", "text/javascript"))

			// Add route
			if err := server.AddRoutes(r, zapLogger); err != nil {
				return errors.Wrap(err, "server.AddRoutes")
			}

			// Add CORS
			c := cors.New(cors.Options{
				AllowedOrigins:   []string{"*"},
				AllowCredentials: true,
				AllowedMethods:   []string{http.MethodPost, http.MethodGet, http.MethodPut, http.MethodDelete},
			})

			port := viper.GetString(portKey)
			if len(port) == 0 {
				port = "80"
			}

			zapLogger.Info(fmt.Sprintf("Server has started with port: %s", port))
			return http.ListenAndServe(fmt.Sprintf(":%s", port), c.Handler(r))
		},
	}

	return command
}

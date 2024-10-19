package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	// Declare a HTTP server using the same settings in our main() function
	// 声明一个HTTP服务器保存地址，处理器，时间戳等信息，并使用mux
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		// 设置http.Server使用标准库中的log.Logger实例，将自定义的Logger作为目标写入目的地
		// 这样http.Server自己的一些日志信息就也被写入JSON中了
		ErrorLog: log.New(app.logger, "", 0),
	}

	// Create a shutdownError channel. Use this shutdownError receive any errors returned
	// by the graceful Shutdown() function
	shutdownError := make(chan error)

	// Start a background goroutine 来捕捉信号并进行Shutdown
	go func() {
		// Create a quit channel which carries os.Signal values
		quit := make(chan os.Signal, 1)

		// Use signal.Notify to listen for incoming SIGINT and SIGTERM signals
		// and rely them to the quit channel
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Read the signal from the quit channel, This code will block until a signal is received
		s := <-quit

		// Log a message to say that the signal has been caught
		app.logger.PrintInfo("shutting down server", map[string]string{
			"signal": s.String(),
		})

		// Create a context with a 5-second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Call Shutdown() on our server passing n the context we just made
		// Shutdown() will return nil if it was successful
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}
		//
		app.logger.PrintInfo("completing background tasks", map[string]string{
			"addr": srv.Addr,
		})

		// Call Wait() to block until our WaitGroup counter is zero,then we return nil on
		// the shutdownError channel, to indicate that the shutdown completed without any issues
		app.wg.Wait()
		shutdownError <- nil
	}()

	// Start the HTTP server
	app.logger.PrintInfo("starting server ", map[string]string{
		"addr": srv.Addr,
		"env":  app.config.env,
	})

	// Calling Shutdown() on our server will cause ListenAndServe() to immediately return
	// a http.ErrServerClosed error. So if we see this,it is actually a good thing
	// So we check specifically for this
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Otherwise, we wait to receive the return value from Shutdown() on the shutdownError channel
	err = <-shutdownError
	if err != nil {
		return err
	}

	// At this point we know that the graceful shutdown completed successfully
	app.logger.PrintInfo("stopped server", map[string]string{
		"addr": srv.Addr,
	})

	return nil
}

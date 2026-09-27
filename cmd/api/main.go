package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rokishi-back/internal/config"
	"rokishi-back/internal/database"
	"rokishi-back/internal/http/router"
	"rokishi-back/internal/repository"
	"rokishi-back/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api detenida", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	locationRepository := repository.NewLocationRepository(pool)
	machineTypeRepository := repository.NewMachineTypeRepository(pool)
	machineRepository := repository.NewMachineRepository(pool)
	machineStateRepository := repository.NewMachineStateRepository(pool)
	materialRepository := repository.NewMaterialRepository(pool)
	rateRepository := repository.NewRateRepository(pool)
	locationService := service.NewLocationService(locationRepository)
	machineTypeService := service.NewMachineTypeService(machineTypeRepository)
	machineService := service.NewMachineService(machineRepository)
	machineStateService := service.NewMachineStateService(machineStateRepository)
	materialService := service.NewMaterialService(materialRepository)
	rateService := service.NewRateService(rateRepository)

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler: router.New(router.Dependencies{
			Ping:           pool.Ping,
			Locations:      locationService,
			MachineTypes:   machineTypeService,
			Machines:       machineService,
			MachineStates:  machineStateService,
			Materials:      materialService,
			Rates:          rateService,
			AllowedOrigins: cfg.AllowedOrigins,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	logger.Info("api iniciada", "addr", listener.Addr().String())

	select {
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("apagando api")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

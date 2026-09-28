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
	"rokishi-back/internal/document"
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
	customerRepository := repository.NewCustomerRepository(pool)
	quoteRepository := repository.NewQuoteRepository(pool)
	productionRepository := repository.NewProductionRepository(pool)
	metricsRepository := repository.NewMetricsRepository(pool)
	securityRepository := repository.NewSecurityRepository(pool)
	locationService := service.NewLocationService(locationRepository)
	machineTypeService := service.NewMachineTypeService(machineTypeRepository)
	machineService := service.NewMachineService(machineRepository)
	machineStateService := service.NewMachineStateService(machineStateRepository)
	materialService := service.NewMaterialService(materialRepository)
	rateService := service.NewRateService(rateRepository)
	customerService := service.NewCustomerService(customerRepository)
	quoteService := service.NewQuoteService(quoteRepository, document.NewQuotePDFGenerator())
	productionService := service.NewProductionService(productionRepository)
	metricsService := service.NewMetricsService(metricsRepository)
	setupCode, err := service.GenerateSetupCode()
	if err != nil {
		return err
	}
	securityService, err := service.NewSecurityService(securityRepository, setupCode, 12*time.Hour)
	if err != nil {
		return err
	}
	logger.Warn("codigo de configuracion inicial; solo funciona mientras no existan usuarios", "codigo_configuracion", setupCode)

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
			Customers:      customerService,
			Quotes:         quoteService,
			Production:     productionService,
			Metrics:        metricsService,
			Security:       securityService,
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

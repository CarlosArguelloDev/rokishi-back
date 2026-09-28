package router

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"rokishi-back/internal/http/handlers"
)

type Dependencies struct {
	Ping           func(context.Context) error
	Locations      handlers.LocationService
	MachineTypes   handlers.MachineTypeService
	Machines       handlers.MachineService
	MachineStates  handlers.MachineStateService
	Materials      handlers.MaterialService
	Rates          handlers.RateService
	Customers      handlers.CustomerService
	Quotes         handlers.QuoteService
	Production     handlers.ProductionService
	Metrics        handlers.MetricsService
	Security       handlers.SecurityService
	AllowedOrigins []string
}

func New(dependencies Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(securityHeadersMiddleware)
	r.Use(corsMiddleware(dependencies.AllowedOrigins))
	r.Get("/api/health", handlers.Health(dependencies.Ping))

	var security *handlers.SecurityHandler
	if dependencies.Security != nil {
		security = handlers.NewSecurityHandler(dependencies.Security)
		r.Get("/api/auth/status", security.Status)
		r.Post("/api/auth/bootstrap", security.Bootstrap)
		r.Post("/api/auth/login", security.Login)
		r.Post("/api/auth/logout", security.Logout)
	}

	r.Group(func(protected chi.Router) {
		protected.Use(authenticationMiddleware(dependencies.Security))
		protected.Use(auditMiddleware(dependencies.Security))

		if security != nil {
			protected.Get("/api/auth/me", security.Me)
			protected.With(requireAdminMiddleware).Get("/api/usuarios", security.ListUsers)
			protected.With(requireAdminMiddleware).Post("/api/usuarios", security.CreateUser)
			protected.With(requireAdminMiddleware).Patch("/api/usuarios/{id}", security.UpdateUser)
			protected.With(requireAdminMiddleware).Get("/api/auditoria", security.ListAudit)
		}

		locations := handlers.NewLocationHandler(dependencies.Locations)
		protected.Post("/api/locaciones", locations.Create)
		protected.Get("/api/locaciones", locations.List)
		protected.Get("/api/locaciones/{id}", locations.Get)
		protected.Patch("/api/locaciones/{id}", locations.Update)

		machineTypes := handlers.NewMachineTypeHandler(dependencies.MachineTypes)
		protected.Post("/api/tipos-maquina", machineTypes.Create)
		protected.Get("/api/tipos-maquina", machineTypes.List)
		protected.Get("/api/tipos-maquina/{id}", machineTypes.Get)
		protected.Patch("/api/tipos-maquina/{id}", machineTypes.Update)

		machines := handlers.NewMachineHandler(dependencies.Machines)
		protected.Post("/api/maquinas", machines.Create)
		protected.Get("/api/maquinas", machines.List)
		protected.Get("/api/maquinas/{id}", machines.Get)
		protected.Patch("/api/maquinas/{id}", machines.Update)

		machineStates := handlers.NewMachineStateHandler(dependencies.MachineStates)
		protected.Get("/api/estados-maquina", machineStates.List)
		protected.Get("/api/maquinas/{id}/estado-actual", machineStates.Current)
		protected.Post("/api/maquinas/{id}/cambios-estado", machineStates.Change)
		protected.Get("/api/maquinas/{id}/historial-estados", machineStates.History)

		materials := handlers.NewMaterialHandler(dependencies.Materials)
		protected.Post("/api/materiales", materials.Create)
		protected.Get("/api/materiales", materials.List)
		protected.Get("/api/materiales/{id}", materials.Get)
		protected.Patch("/api/materiales/{id}", materials.Update)

		rates := handlers.NewRateHandler(dependencies.Rates)
		protected.Get("/api/maquinas/{id}/tarifa", rates.GetMachine)
		protected.Put("/api/maquinas/{id}/tarifa", rates.PutMachine)
		protected.Get("/api/locaciones/{id}/tarifa-energia", rates.GetEnergy)
		protected.Put("/api/locaciones/{id}/tarifa-energia", rates.PutEnergy)

		customers := handlers.NewCustomerHandler(dependencies.Customers)
		protected.Post("/api/clientes", customers.Create)
		protected.Get("/api/clientes", customers.List)
		protected.Get("/api/clientes/{id}", customers.Get)
		protected.Patch("/api/clientes/{id}", customers.Update)

		quotes := handlers.NewQuoteHandler(dependencies.Quotes)
		protected.Post("/api/cotizaciones/calcular", quotes.Calculate)
		protected.Post("/api/cotizaciones", quotes.Create)
		protected.Get("/api/cotizaciones", quotes.List)
		protected.Get("/api/cotizaciones/{id}", quotes.Get)
		protected.Post("/api/cotizaciones/{id}/pdf", quotes.GeneratePDF)
		protected.Get("/api/estados-cotizacion", quotes.ListStatuses)
		protected.Post("/api/cotizaciones/{id}/cambios-estado", quotes.ChangeStatus)

		production := handlers.NewProductionHandler(dependencies.Production)
		protected.Post("/api/cotizaciones/{id}/pedido", production.CreateOrder)
		protected.Post("/api/pedidos", production.CreateDirectOrder)
		protected.Get("/api/pedidos", production.ListOrders)
		protected.Get("/api/pedidos/{id}", production.GetOrder)
		protected.Patch("/api/trabajos/{id}/asignacion", production.AssignMachine)
		protected.Post("/api/trabajos/{id}/iniciar", production.StartWork)
		protected.Post("/api/trabajos/{id}/finalizar", production.FinishWork)

		metrics := handlers.NewMetricsHandler(dependencies.Metrics)
		protected.Get("/api/metricas/resumen", metrics.Summary)
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusNotFound, "not_found", "Ruta no encontrada")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Metodo no permitido")
	})
	return r
}

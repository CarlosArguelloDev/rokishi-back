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
	AllowedOrigins []string
}

func New(dependencies Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(corsMiddleware(dependencies.AllowedOrigins))
	r.Get("/api/health", handlers.Health(dependencies.Ping))

	locations := handlers.NewLocationHandler(dependencies.Locations)
	r.Post("/api/locaciones", locations.Create)
	r.Get("/api/locaciones", locations.List)
	r.Get("/api/locaciones/{id}", locations.Get)
	r.Patch("/api/locaciones/{id}", locations.Update)

	machineTypes := handlers.NewMachineTypeHandler(dependencies.MachineTypes)
	r.Post("/api/tipos-maquina", machineTypes.Create)
	r.Get("/api/tipos-maquina", machineTypes.List)
	r.Get("/api/tipos-maquina/{id}", machineTypes.Get)
	r.Patch("/api/tipos-maquina/{id}", machineTypes.Update)

	machines := handlers.NewMachineHandler(dependencies.Machines)
	r.Post("/api/maquinas", machines.Create)
	r.Get("/api/maquinas", machines.List)
	r.Get("/api/maquinas/{id}", machines.Get)
	r.Patch("/api/maquinas/{id}", machines.Update)

	machineStates := handlers.NewMachineStateHandler(dependencies.MachineStates)
	r.Get("/api/estados-maquina", machineStates.List)
	r.Get("/api/maquinas/{id}/estado-actual", machineStates.Current)
	r.Post("/api/maquinas/{id}/cambios-estado", machineStates.Change)
	r.Get("/api/maquinas/{id}/historial-estados", machineStates.History)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusNotFound, "not_found", "Ruta no encontrada")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Metodo no permitido")
	})
	return r
}

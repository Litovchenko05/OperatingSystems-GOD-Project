package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/sisoputnfrba/tp-golang/cpu/cicloDeInstruccion"
	"github.com/sisoputnfrba/tp-golang/cpu/utils"
	"github.com/sisoputnfrba/tp-golang/utils/conexiones"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

var mu1 sync.Mutex

func Inicializar_cpu(logger *slog.Logger) {
	mux := http.NewServeMux()

	// Endpoints de kernel
	mux.HandleFunc("POST /EJECUTAR_KERNEL", Recibir_PIDTID(logger))
	mux.HandleFunc("POST /INTERRUPCION_FIN_QUANTUM", RecibirInterrupcion(logger))
	mux.HandleFunc("POST /PRIORIDAD", RecibirInterrupcion(logger))
	//mux.HandleFunc("POST /comunicacion-memoria", ComunicacionMemoria(logger))

	conexiones.LevantarServidor(strconv.Itoa(utils.Configs.Port), mux, logger)

}

// SetGlobalPIDTID recibe un PIDTID y actualiza las variables globales PID y TID.
func Recibir_PIDTID(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var pidtid types.PIDTID

		// Decodificar el cuerpo de la solicitud JSON
		if err := json.NewDecoder(r.Body).Decode(&pidtid); err != nil {
			http.Error(w, "Error al decodificar el JSON de la solicitud", http.StatusBadRequest)
			logger.Error("Error al decodificar JSON", slog.String("error", err.Error()))
			return
		}
		mu1.Lock()
		// Almacenar el PID y TID en la variable global
		cicloDeInstruccion.GlobalPIDTID = pidtid

		// Log de confirmación de la actualización
		logger.Info("PID y TID actualizados", slog.Any(
			"PID", pidtid.PID), slog.Any("TID", pidtid.TID))

		utils.Control = true
		// Llamar a Comenzar_cpu para iniciar el proceso de CPU
		cicloDeInstruccion.Comenzar_cpu(logger)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("PID y TID almacenados y CPU iniciada"))
		mu1.Unlock()
	}
}

// Función para recibir la interrupción y el TID desde la solicitud
func RecibirInterrupcion(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar que el cuerpo de la solicitud no sea nulo
		if r.Body == nil {
			logger.Error("Cuerpo vacío en la solicitud")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Cuerpo vacío"))
			return
		}
		defer r.Body.Close() // Asegurarse de cerrar el cuerpo de la solicitud

		// Decodificar el JSON recibido
		var bodyInterrupcion types.InterruptionInfo
		err := json.NewDecoder(r.Body).Decode(&bodyInterrupcion)
		if err != nil {
			logger.Error("Error al decodificar mensaje", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error al decodificar mensaje"))
			return
		}
		// Log del mensaje recibido
		logger.Debug("Interrupción recibida", slog.Any("InterruptionInfo", bodyInterrupcion))

		// Almacenar la interrupción recibida en la variable global
		interrupcion := &types.InterruptionInfo{
			NombreInterrupcion: bodyInterrupcion.NombreInterrupcion,
			TID:                bodyInterrupcion.TID,
			PID:                bodyInterrupcion.PID,
		}

		cicloDeInstruccion.InterrupcionRecibida = interrupcion

		// Log de confirmación
		logger.Info("## Llega interrupción al puerto Interrupt",
			slog.String("NombreInterrupcion", bodyInterrupcion.NombreInterrupcion),
			slog.Int("PID", int(bodyInterrupcion.PID)),
			slog.Int("TID", int(bodyInterrupcion.TID)),
		)

		// Responder con éxito
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Interrupción y TID almacenados"))
	}
}

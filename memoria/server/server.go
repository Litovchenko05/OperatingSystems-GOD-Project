package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/sisoputnfrba/tp-golang/memoria/client"
	"github.com/sisoputnfrba/tp-golang/memoria/memSistema"
	"github.com/sisoputnfrba/tp-golang/memoria/memUsuario"
	"github.com/sisoputnfrba/tp-golang/memoria/utils"
	"github.com/sisoputnfrba/tp-golang/utils/conexiones"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

func Iniciar_memoria(logger *slog.Logger) {
	mux := http.NewServeMux()

	// Comuniacion con Kernel
	mux.HandleFunc("POST /CREAR-PROCESO", Crear_proceso(logger))
	mux.HandleFunc("PATCH /FINALIZAR-PROCESO/{pid}", FinalizarProceso(logger))
	mux.HandleFunc("POST /CREAR_HILO", Crear_hilo(logger))
	mux.HandleFunc("POST /FINALIZAR_HILO", FinalizarHilo(logger))
	mux.HandleFunc("POST /MEMORY-DUMP", MemoryDump(logger))
	mux.HandleFunc("POST /compactar", Compactar(logger))

	// Comunicacion con CPU
	mux.HandleFunc("POST /contexto", Obtener_Contexto_De_Ejecucion(logger))
	mux.HandleFunc("POST /actualizar_contexto", Actualizar_Contexto(logger))
	mux.HandleFunc("GET /instruccion", Obtener_Instrucción(logger))
	mux.HandleFunc("POST /read_mem", Read_Mem(logger))
	mux.HandleFunc("POST /write_mem", Write_Mem(logger))

	conexiones.LevantarServidor(strconv.Itoa(utils.Configs.Port), mux, logger)

}

func Crear_proceso(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var magic types.PathTamanio
		err := decoder.Decode(&magic)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error al decodificar mensaje"))
			return
		}
		logger.Info(fmt.Sprintf("Me llegaron los siguientes parametros para crear proceso: %+v", magic))

		// Llamar a Inicializar_proceso con los parámetros correspondientes
		sePudo, msj := memUsuario.AsignarPID(magic.PID, magic.Tamanio, magic.Path, logger)

		// Si la inicialización fue exitosa
		if sePudo {
			logger.Info(fmt.Sprintf("## Proceso Creado - PID: %d  - Tamaño: %d", magic.PID, magic.Tamanio))
			w.WriteHeader(http.StatusOK)
			return
		}
		if !sePudo && (msj == "NO SE PUDO INICIALIZAR EL PROCESO POR FALTA DE HUECOS EN LAS PARTICIONES") {
			logger.Info(msj)
			w.WriteHeader(http.StatusInsufficientStorage)
			return
		}

		if !sePudo && (msj == "COMPACTACION") {
			logger.Info("se puede compactar")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte("COMPACTACION"))
			return
		}
		w.WriteHeader(http.StatusConflict)

	}
}

func FinalizarProceso(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar que el método sea PATCH
		if r.Method != http.MethodPatch {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}

		// Decodificar la solicitud para obtener el PID
		param := r.PathValue("pid")
		pid, err := strconv.ParseUint(param, 10, 32)
		if err != nil {
			fmt.Println("Error al convertir:", err)
			return
		}
		pidUint32 := uint32(pid)

		//marca la particion como libre en memoria de usuario
		memUsuario.LiberarParticionPorPID(pidUint32, logger)
		// Ejecutar la función para eliminar el contexto del PID en Memoria de sistema
		memSistema.EliminarContextoPID(pidUint32)
		// Log de destrucción del proceso
		logger.Info(fmt.Sprintf("## Proceso Destruido - PID: %d", pidUint32))

		// Responder al Kernel con "OK" si la operación fue exitosa
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func Crear_hilo(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		decoder := json.NewDecoder(r.Body)
		var magic types.EnviarHiloAMemoria
		err := decoder.Decode(&magic)
		if err != nil {
			logger.Error(fmt.Sprintf("Error al decodificar mensaje: %s\n", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Error al decodificar mensaje"))
			return
		}
		logger.Info(fmt.Sprintf("Me llegaron los siguientes parametros para crear proceso: %+v", magic))

		memSistema.CrearContextoTID(magic.PID, magic.TID, magic.Path)

		logger.Info(fmt.Sprintf("## Hilo Creado - (PID:TID) - (%d:%d)", magic.PID, magic.PID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// hecho
func FinalizarHilo(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar que el método sea POST
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}

		// Decodificar la solicitud para obtener el PID y TID
		var pidTid struct {
			PID uint32 `json:"pid"`
			TID uint32 `json:"tid"`
		}
		err := json.NewDecoder(r.Body).Decode(&pidTid)
		if err != nil {
			http.Error(w, "Error al decodificar la solicitud", http.StatusBadRequest)
			return
		}

		// Log de solicitud de finalización del hilo
		logger.Info(fmt.Sprintf("## Finalizar hilo solicitado - (PID:TID) - (%d:%d)", pidTid.PID, pidTid.TID))

		// Ejecutar la función para eliminar el contexto del TID en Memoria
		memSistema.EliminarContextoTID(pidTid.PID, pidTid.TID)

		// Log de destrucción del hilo
		logger.Info(fmt.Sprintf("## Hilo Destruido - (PID:TID) - (%d:%d)", pidTid.PID, pidTid.TID))

		// Responder al Kernel con "OK" si la operación fue exitosa
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}
}

// Función que maneja el endpoint de Memory Dump a partir del archivo recibido por file System
func MemoryDump(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}

		// Decodificar la solicitud para obtener el PID y TID
		var pidTid struct {
			TID uint32 `json:"tid"`
			PID uint32 `json:"pid"`
		}

		err := json.NewDecoder(r.Body).Decode(&pidTid)
		if err != nil {
			logger.Error("Error al decodificar la solicitud", slog.Any("error", err))
			http.Error(w, "Error al decodificar la solicitud", http.StatusBadRequest)
			return
		}

		logger.Info(fmt.Sprintf("## Memory Dump Solicitado - (PID:TID) - (%d:%d) ", pidTid.PID, pidTid.TID))
		//creo la variable para guardar la memoria del proceso que se envia a filesystem
		var memoriaProceso []byte

		// Verificar el esquema de memoria
		if utils.Configs.Scheme == "DINAMICAS" {
			// Buscar el proceso en memoria dinámica
			posicion, existe := memUsuario.PidAParticion[pidTid.PID]
			if !existe {
				logger.Error("PID no encontrado en memoria dinámica", slog.Any("pid", pidTid.PID))
				http.Error(w, "PID no encontrado", http.StatusNotFound)
				return
			}

			// Calcular base y tamaño de la partición
			base := memUsuario.BaseDinamica(posicion)
			tamanio := uint32(memUsuario.ParticionesDinamicas[posicion])

			// Extraer la memoria del proceso
			memoriaProceso = memUsuario.MemoriaDeUsuario[base : base+tamanio]

			// Si el esquema es fijo
		} else if utils.Configs.Scheme == "FIJAS" {
			// Buscar el proceso en memoria de particiones fijas
			particion, existe := memUsuario.PidAParticion[pidTid.PID]
			if !existe {
				logger.Error("PID no encontrado en memoria fija", slog.Any("pid", pidTid.PID))
				http.Error(w, "PID no encontrado", http.StatusNotFound)
				return
			}

			// Extraer la memoria del proceso
			memoriaProceso = memUsuario.MemoriaDeUsuario[memUsuario.Particiones[particion].Base : memUsuario.Particiones[particion].Base+memUsuario.Particiones[particion].Limite]
		}

		// Generar el timestamp actual
		timestamp := time.Now().Unix()

		// Crear la estructura con la memoria, timestamp, PID y TID
		memoryDumpRequest := types.DumpFile{
			Nombre:  fmt.Sprintf("%d-%d-%d.dmp", pidTid.PID, pidTid.TID, timestamp),
			Tamanio: len(memoriaProceso),
			Datos:   memoriaProceso,
		}

		// Enviar la estructura al FileSystem para crear el archivo con el dump
		exito := client.Enviar_Body(memoryDumpRequest, utils.Configs.IpFilesystem, utils.Configs.PortFilesystem, "dump", logger)

		// Si el dump no se pudo realizar:
		if !exito {
			logger.Error("Error al enviar el dump al FileSystem")
			respuesta := types.RespuestaDump{
				PID:       pidTid.PID,
				TID:       pidTid.TID,
				Respuesta: "Error al enviar el dump al FileSystem",
			}
			client.Enviar_Body(respuesta, utils.Configs.IpKernel, utils.Configs.PortKernel, "dump_response", logger)
			return
		}

		// Si el dump se realizo con éxito:
		respuestaOK := types.RespuestaDump{
			PID:       pidTid.PID,
			TID:       pidTid.TID,
			Respuesta: "OK",
		}
		client.Enviar_Body(respuestaOK, utils.Configs.IpKernel, utils.Configs.PortKernel, "dump_response", logger)
	}
}

func Compactar(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("## Compactación Solicitada")
		if memUsuario.Compactar() {
			logger.Info("## Compactación Realizada")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			logger.Error("NO SE HAN ENCONTRADO HUECOS LIBRES PARA PODER COMPACTAR")
		}
	}
}

// Función que envia el contexto del pid y tid a cpu
func Obtener_Contexto_De_Ejecucion(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		retardoDePeticion()
		// Decodificar la solicitud para obtener el PID y TID
		var pidTid types.PIDTID // Mover la estructura a un paquete compartido si es común
		err := json.NewDecoder(r.Body).Decode(&pidTid)
		if err != nil {
			logger.Error("Error al decodificar la solicitud", slog.Any("error", err))
			http.Error(w, "Error al decodificar la solicitud", http.StatusBadRequest)
			return
		}

		// Buscar el contexto para el PID en el mapa ContextosPID
		contextoPID, existePID := memSistema.ContextosPID[pidTid.PID]
		if !existePID {
			logger.Error(fmt.Sprintf("PID %d no encontrado", pidTid.PID))
			http.Error(w, "PID no encontrado", http.StatusNotFound)
			return
		}

		// Buscar el TID dentro del contexto del PID
		contextoTID, existeTID := contextoPID.TIDs[pidTid.TID]
		if !existeTID {
			logger.Error(fmt.Sprintf("TID %d no encontrado en el PID %d", pidTid.TID, pidTid.PID))
			http.Error(w, "TID no encontrado en el PID", http.StatusNotFound)
			return
		}

		// Log de solicitud de contexto OBLIGATORIO
		logger.Info(fmt.Sprintf("## Contexto Solicitado - (PID:TID) - (%d:%d)", pidTid.PID, pidTid.TID))

		// Crear el contexto completo usando la estructura que CPU espera (RegCPU)
		contextoCompleto := types.RegCPU{
			PC:     contextoTID.PC,
			AX:     contextoTID.AX,
			BX:     contextoTID.BX,
			CX:     contextoTID.CX,
			DX:     contextoTID.DX,
			EX:     contextoTID.EX,
			FX:     contextoTID.FX,
			GX:     contextoTID.GX,
			HX:     contextoTID.HX,
			Base:   contextoPID.Base,
			Limite: contextoPID.Limite,
		}

		// Codificar el contexto completo como JSON y enviarlo como respuesta
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(contextoCompleto)
		if err != nil {
			logger.Error("Error al codificar la respuesta", slog.Any("error", err))
			http.Error(w, "Error al codificar la respuesta", http.StatusInternalServerError)
			return
		}
		logger.Info(fmt.Sprintf("Contexto completo enviado para PID %d y TID %d", pidTid.PID, pidTid.TID))
	}
}

func Actualizar_Contexto(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		retardoDePeticion()
		var req types.Proceso
		err := json.NewDecoder(r.Body).Decode(&req)

		contexto := types.ContextoEjecucionTID{
			PC:                 req.ContextoEjecucion.PC, // Program Counter (Proxima instruccion a ejecutar)
			AX:                 req.ContextoEjecucion.AX, // Acumulador
			BX:                 req.ContextoEjecucion.BX, // Base
			CX:                 req.ContextoEjecucion.CX, // Contador
			DX:                 req.ContextoEjecucion.DX, // Datos
			EX:                 req.ContextoEjecucion.EX, // Extra
			FX:                 req.ContextoEjecucion.FX, // Flag
			GX:                 req.ContextoEjecucion.GX, // General
			HX:                 req.ContextoEjecucion.HX, // General
			LISTAINSTRUCCIONES: memSistema.ContextosPID[req.Pid].TIDs[req.Tid].LISTAINSTRUCCIONES,
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		memSistema.Actualizar_TID(req.Pid, req.Tid, contexto)
		logger.Info(fmt.Sprintf("## Contexto Actualizado - (PID:TID) - (%d:%d) ", req.Pid, req.Tid))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))

	}
}

func Obtener_Instrucción(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		retardoDePeticion()
		var requestData struct {
			PC  uint32 `json:"pc"`
			TID uint32 `json:"tid"`
			PID uint32 `json:"pid"`
		}
		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			// Si hay error al decodificar la solicitud, enviar una respuesta con error
			http.Error(w, fmt.Sprintf("Error al leer la solicitud: %v", err), http.StatusBadRequest)
			return
		}

		instruccion := memSistema.BuscarSiguienteInstruccion(requestData.PID, requestData.TID, requestData.PC)
		logger.Info(fmt.Sprintf("## OBTENER INSTRUCCION -(PID:TID) -(%d:%d) - Instruccion: %s", requestData.PID, requestData.TID, instruccion))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(instruccion))
	}
}

func Read_Mem(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		retardoDePeticion()
		// Crear una estructura para la solicitud que contiene la dirección física
		var requestData struct {
			DireccionFisica uint32 `json:"direccion_fisica"`
			TID             uint32 `json:"tid"`
			PID             uint32 `json:"pid"`
		}

		// Decodificar el cuerpo de la solicitud JSON
		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			// Si hay error al decodificar la solicitud, enviar una respuesta con error
			http.Error(w, fmt.Sprintf("Error al leer la solicitud: %v", err), http.StatusBadRequest)
			return
		}

		// Verificar que la dirección esté dentro de los límites de memoria
		if requestData.DireccionFisica+4 > uint32(len(memUsuario.MemoriaDeUsuario)) {
			// Si hay error al buscar en la memoria, enviar una respuesta con error
			http.Error(w, fmt.Sprintf("Dirección fuera de los límites de memoria. Dirección solicitada: %d", requestData.DireccionFisica), http.StatusBadRequest)
			return
		}

		// Obtener los 4 bytes desde la dirección solicitada
		bytes := memUsuario.MemoriaDeUsuario[requestData.DireccionFisica : requestData.DireccionFisica+4]

		// Convertir los 4 bytes a uint32 (Little Endian)
		valor := binary.LittleEndian.Uint32(bytes)

		// Agregar log de lectura en espacio de usuario
		logger.Info(fmt.Sprintf("## LEER - (%d:%d) - (%d:%d) - Dir. Física: %d - Tamaño: %d",
			requestData.PID, requestData.TID, requestData.PID, requestData.TID, requestData.DireccionFisica, 4)) // Tamaño de lectura: 4 bytes

		// Crear la respuesta JSON con el valor leído
		responseData := struct {
			Valor uint32 `json:"valor"`
		}{
			Valor: valor,
		}

		// Serializar la respuesta en JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(responseData); err != nil {
			// Si hay error al serializar la respuesta, responder con error
			http.Error(w, fmt.Sprintf("Error al enviar la respuesta: %v", err), http.StatusInternalServerError)
		}
	}
}

func Write_Mem(logger *slog.Logger) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		retardoDePeticion()
		var requestData struct {
			DireccionFisica uint32 `json:"direccion_fisica"`
			Valor           uint32 `json:"valor"`
			TID             uint32 `json:"tid"`
		}

		// Decodificar el JSON
		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			logger.Error("Error al decodificar JSON en Write_Mem", slog.Any("error", err))
			http.Error(w, "Error al decodificar JSON", http.StatusBadRequest)
			return
		}

		// Verificar si la dirección física está dentro de alguna partición
		encontrado := false
		for _, particion := range memUsuario.Particiones {
			if requestData.DireccionFisica >= particion.Base && requestData.DireccionFisica < particion.Base+particion.Limite {
				encontrado = true
				break
			}
		}
		if !encontrado {
			logger.Error("Dirección física fuera de rango de particiones")
			http.Error(w, "Dirección física fuera de rango de particiones", http.StatusBadRequest)
			return
		}

		// Verificar que la dirección esté dentro de los límites de memoria
		if int(requestData.DireccionFisica+4) > len(memUsuario.MemoriaDeUsuario) {
			logger.Error("Dirección física fuera de los límites de la memoria", slog.Any("direccion_fisica", requestData.DireccionFisica))
			http.Error(w, "Dirección fuera de los límites de memoria", http.StatusBadRequest)
			return
		}

		// Escribir el valor en little-endian en la memoria
		binary.LittleEndian.PutUint32(memUsuario.MemoriaDeUsuario[requestData.DireccionFisica:], requestData.Valor)

		// Log obligatorio de Escritura en espacio de usuario
		logger.Info(fmt.Sprintf("## Escritura - (PID:TID) - (N/A:%d) - Dir. Física: %d - Tamaño: %d",
			requestData.TID, requestData.DireccionFisica, 4))

		// Confirmar la operación
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))

		// Log de escritura exitosa
		logger.Info(fmt.Sprintf("Escritura en memoria de usuario exitosa: TID %d - Dirección Física: %d - Valor: %d- Tamaño: %d",
			requestData.TID, requestData.DireccionFisica, requestData.Valor, 4))
	}

}

// A partir del tiempo que nos pasa el archivo configs esperamos esa cantidad en milisegundos antes de seguir con la ejecucion del proceso
func retardoDePeticion() {
	time.Sleep(time.Duration(utils.Configs.ResponseDelay) * time.Millisecond)
}

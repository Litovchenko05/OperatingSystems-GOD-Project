package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/sisoputnfrba/tp-golang/cpu/utils"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

// Variable global para almacenar el contexto de ejecución
var ReceivedContextoEjecucion *types.RegCPU = nil

func SolicitarContextoEjecucion(pidTid types.PIDTID, logger *slog.Logger) error {
	// Codificar el dato
	body, err := json.Marshal(pidTid)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje", slog.Any("error", err))
		return fmt.Errorf("error al codificar PIDTID a JSON: %w", err)
	}

	// Construir la URL del endpoint usando las configuraciones globales
	endpoint := "contexto"
	url := fmt.Sprintf("http://%s:%d/%s", utils.Configs.IpMemory, utils.Configs.PortMemory, endpoint)

	// Realizar la solicitud POST
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error("Se produjo un error enviando mensaje al módulo de memoria", slog.Any("error", err))
		return fmt.Errorf("error al enviar solicitud al módulo de memoria: %w", err)
	}
	defer resp.Body.Close() // Asegurar el cierre del body

	// Validar el código de estado
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("La respuesta del servidor no fue OK. Código: %d", resp.StatusCode))
		return fmt.Errorf("respuesta del servidor no fue OK: %d", resp.StatusCode)
	}

	// Decodificar el cuerpo de la respuesta
	var contexto types.RegCPU
	err = json.NewDecoder(resp.Body).Decode(&contexto)
	if err != nil {
		logger.Error("Error al decodificar el contexto de ejecución", slog.Any("error", err))
		return fmt.Errorf("error al decodificar el cuerpo de la respuesta: %w", err)
	}

	// Guardar el contexto recibido en la variable global
	ReceivedContextoEjecucion = &contexto

	Proceso.ContextoEjecucion = contexto

	logger.Info("Contexto de ejecución recibido con éxito")
	return nil
}

var Proceso types.Proceso

// creo que ya no la usa nadie
func DevolverTIDAlKernel(tid uint32, logger *slog.Logger, endpoint string, motivo string) bool {
	cliente := &http.Client{}
	url := fmt.Sprintf("http://%s:%d/%s/%v", utils.Configs.IpKernel, utils.Configs.PortKernel, endpoint, tid)
	req, err := http.NewRequest("PATCH", url, nil)
	if err != nil {
		return false
	}
	// Establecer el encabezado Content-Type
	req.Header.Set("Content-Type", "application/json")

	// Enviar la solicitud
	resp, err := cliente.Do(req)
	if err != nil {
		logger.Error(fmt.Sprintf("Error al enviar la solicitud: %v", err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return false // Indica que la respuesta no fue exitosa
	}

	return true
}

func EnviarContextoDeEjecucion[T any](dato T, endpoint string, logger *slog.Logger) bool {

	body, err := json.Marshal(dato)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje")
		return false
	}
	//ipMemory y portMemory
	url := fmt.Sprintf("http://%s:%d/%s", utils.Configs.IpMemory, utils.Configs.PortMemory, endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Se produjo un error enviando mensaje a ip:%s puerto:%d", utils.Configs.IpMemory, utils.Configs.PortMemory))
		return false
	}
	// Aseguramos que el body sea cerrado
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return false // Indica que la respuesta no fue exitosa
	}
	return true // Indica que la respuesta fue exitosa
}

func CederControlAKernell[T any](dato T, endpoint string, logger *slog.Logger) {

	body, err := json.Marshal(dato)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje")
		return
	}

	url := fmt.Sprintf("http://%s:%d/%s", utils.Configs.IpKernel, utils.Configs.PortKernel, endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Se produjo un error enviando mensaje a ip:%s puerto:%d", utils.Configs.IpKernel, utils.Configs.PortKernel))
		return
	}
	// Aseguramos que el body sea cerrado
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return // Indica que la respuesta no fue exitosa
	}
}

// EnviarDesalojo envia el PID, TID y el motivo del desalojo a la API Kernel utilizando la configuración global de IP y puerto.
func EnviarDesalojo(pid uint32, tid uint32, motivo string, logger *slog.Logger) {

	//finalizar cpu
	utils.Control = false

	// Crear el objeto que contiene los datos a enviar
	hiloDesalojado := types.HiloDesalojado{
		PID:    pid,
		TID:    tid,
		Motivo: motivo,
	}

	// Convertir el objeto a JSON
	body, err := json.Marshal(hiloDesalojado)
	if err != nil {
		logger.Error("Error al codificar mensaje de desalojo", slog.String("error", err.Error()))
		return
	}

	// Formar la URL de la API Kernel usando las configuraciones globales
	url := fmt.Sprintf("http://%s:%d/recibir-desalojo", utils.Configs.IpKernel, utils.Configs.PortKernel)

	// Enviar la solicitud POST
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Error al enviar desalojo a %s:%d", utils.Configs.IpKernel, utils.Configs.PortKernel), slog.String("error", err.Error()))
		return
	}
	defer resp.Body.Close()

	// Verificar que la respuesta sea exitosa
	if resp.StatusCode != http.StatusOK {
		logger.Error("Error al procesar la solicitud de desalojo", slog.Int("status", resp.StatusCode))
		return
	}

	// Log de éxito
	logger.Info("Desalojo enviado correctamente", slog.Int("PID", int(pid)), slog.Int("TID", int(tid)), slog.String("Motivo", motivo))
}

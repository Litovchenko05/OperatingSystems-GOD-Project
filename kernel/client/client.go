package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Devuelve true en caso de que la respuesta del servidor sea exitosa, false en caso contrario
func Enviar_Body[T any](dato T, ip string, puerto int, endpoint string, logger *slog.Logger) bool {

	body, err := json.Marshal(dato)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje")
		return false
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Se produjo un error enviando mensaje a ip:%s puerto:%d", ip, puerto))
		return false
	}
	// Aseguramos que el body sea cerrado
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("La respuesta del servidor no fue OK")
		return false // Indica que la respuesta no fue exitosa
	}

	// Log de éxito
	logger.Info("Mensaje enviado con éxito",
		slog.Int("puerto", puerto),
		slog.String("endpoint", endpoint),
	)

	return true // Indica que la respuesta fue exitosa
}

func Enviar_Body_Async[T any](dato T, ip string, puerto int, endpoint string, logger *slog.Logger) {
	go func() {
		body, err := json.Marshal(dato)
		if err != nil {
			logger.Error("Se produjo un error codificando el mensaje")
			return
		}

		url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, endpoint)
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			logger.Error(fmt.Sprintf("Error creando la solicitud HTTP: %s", err.Error()))
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error(fmt.Sprintf("Error enviando mensaje: %s", err.Error()))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Error("La respuesta del servidor no fue OK")
			return
		}
	}()
}

// IMPORTANTE! Es QueryPath, no se le pasa un Body
func Enviar_QueryPath[T any](dato T, ip string, puerto int, endpoint string, verbo string, logger *slog.Logger) bool {
	cliente := &http.Client{}
	url := fmt.Sprintf("http://%s:%d/%s/%v", ip, puerto, endpoint, dato)
	req, err := http.NewRequest(verbo, url, nil)
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

	return true // Indica que la respuesta fue exitosa
}

func Enviar_Proceso[T any](dato T, ip string, puerto int, endpoint string, logger *slog.Logger) (bool, string) {

	body, err := json.Marshal(dato)
	if err != nil {
		logger.Error("Se produjo un error codificando el mensaje")
		return false, ""
	}

	url := fmt.Sprintf("http://%s:%d/%s", ip, puerto, endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Error(fmt.Sprintf("Se produjo un error enviando mensaje a ip:%s puerto:%d", ip, puerto))
		return false, ""
	}
	// Aseguramos que el body sea cerrado
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusInsufficientStorage {
		return false, "NO HAY MEMORIA"
	}

	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Se produjo un error leyendo el cuerpo de la respuesta")
			return false, ""
		}
		if string(respBody) == "COMPACTACION" {
			logger.Error("me llego msg de compactacion")
			return false, "COMPACTACION" // Indica que la respuesta no fue exitosa y que ademas memoria solicita compactacion
		}

		logger.Error("La respuesta del servidor no fue OK")
		return false, "" // Indica que la respuesta no fue exitosa
	}

	return true, "" // Indica que la respuesta fue exitosa
}

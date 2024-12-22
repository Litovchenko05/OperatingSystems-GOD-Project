package conexiones

// Importamos librerias
import (
	"fmt"
	"log/slog"
	"net/http"
)

// Recibe por parametro el puerto, el handler y el logger
func LevantarServidor(port string, handler http.Handler, logger *slog.Logger) {
	logger.Info(fmt.Sprintf("Levantando servidor en el puerto: %s", port))
	err := http.ListenAndServe(":"+port, handler)

	//Manejo de errores
	if err != nil {
		logger.Error("Error al levantar el servidor: ", "error", err)
	}
}

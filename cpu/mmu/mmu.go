package mmu

import (
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/sisoputnfrba/tp-golang/cpu/client"
	"github.com/sisoputnfrba/tp-golang/utils/types"
)

func TraducirDireccion(proceso *types.Proceso, direccionLogica uint32, logger *slog.Logger) (uint32, error) {

	direccionFisica := proceso.ContextoEjecucion.Base + direccionLogica
	if direccionFisica >= proceso.ContextoEjecucion.Base+proceso.ContextoEjecucion.Limite {

		proceso.ContextoEjecucion.PC++
		client.EnviarContextoDeEjecucion(proceso, "actualizar_contexto", logger)
		logger.Info(fmt.Sprintf("## TID: %d - Actualizo Contexto Ejecución", proceso.Tid))
		client.EnviarDesalojo(proceso.Pid, proceso.Tid, "SEGMENTATION FAULT", logger)
		log.Printf("Segmentation Fault en Tid %d", proceso.Tid)

		return 0, errors.New("segmentation fault")
	}

	return direccionFisica, nil
}
